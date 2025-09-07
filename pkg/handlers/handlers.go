package handlers

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/Redarek/go-tg-bot-rest/pkg/config"
	"github.com/Redarek/go-tg-bot-rest/pkg/models"
	"github.com/Redarek/go-tg-bot-rest/pkg/repositories"
	"github.com/Redarek/go-tg-bot-rest/pkg/services"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"strconv"
	"strings"
	"time"
)

//go:embed assets/start.jpg
var StartJPG []byte

type Handler struct {
	bot            *tgbotapi.BotAPI
	sender         *services.Sender
	service        *services.Service
	adminID        int64
	shopURL        string
	subChannelID   int64
	subChannelLink string
}

func NewHandler(bot *tgbotapi.BotAPI, sender *services.Sender, db *pgxpool.Pool, cfg *config.Config) *Handler {
	repo := repositories.NewRepository(db)
	return &Handler{
		bot:            bot,
		sender:         sender,
		service:        services.NewService(repo),
		adminID:        cfg.AdminID,
		shopURL:        cfg.ShopURL,
		subChannelID:   cfg.SubChannelID,
		subChannelLink: cfg.SubChannelLink,
	}
}

func (h *Handler) HandleUpdate(upd tgbotapi.Update) {
	// базовый контекст на обработку одного апдейта
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	switch {
	case upd.Message != nil:
		m := upd.Message

		// Сначала админские команды
		if m.IsCommand() && m.From != nil && m.From.ID == h.adminID {
			h.handleAdminCommand(ctx, m)
			return
		}

		// Пользовательские команды
		if m.IsCommand() && m.From != nil && m.From.ID != h.adminID {
			switch m.Command() {
			case "draw":
				h.processDraw(ctx, m.Chat.ID, m.From.ID)
				return
			case "start":
				h.sendStartMessage(ctx, m.Chat.ID)
				return
			}
		}

		// Диалог админа — только для админа (чтобы не бить БД по каждому юзеру)
		if m.From != nil && m.From.ID == h.adminID {
			h.handleAdminDialog(ctx, m)
		}

	case upd.CallbackQuery != nil:
		h.handleCallback(ctx, upd.CallbackQuery)
	}
}

func (h *Handler) sendStartMessage(ctx context.Context, chatID int64) {
	dbctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if err := h.service.Repo.UpsertBotUser(dbctx, chatID); err != nil {
		log.Println("UpsertBotUser:", err)
	}

	mk := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Получить скидку", "draw"),
		))

	caption := "🍀<b><u>Готов испытать удачу?</u></b>\n" +
		"Запускай «Фортуну Вкуса» и забирай случайную скидку в нашем ресторане!\n" +
		"😋<i>Получи скидку и приходи за своим вкусным бонусом!</i>"

	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: "start.jpg", Bytes: StartJPG})
	photo.Caption = caption
	photo.ReplyMarkup = mk
	photo.ParseMode = tgbotapi.ModeHTML
	if _, err := h.sender.Send(ctx, photo); err != nil {
		log.Println("sendStartMessage:", err)
	}
}

func (h *Handler) handleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	// всегда отвечаем на callback, чтобы убрать "часики"
	if q.ID != "" {
		_, _ = h.bot.Request(tgbotapi.NewCallback(q.ID, ""))
	}

	// Бывают инлайн-коллбэки без Message
	if q.Message == nil {
		return
	}

	switch {
	case q.Data == "start":
		h.sendStartMessage(ctx, q.Message.Chat.ID)

	case q.Data == "draw":
		h.processDraw(ctx, q.Message.Chat.ID, q.From.ID)

	case strings.HasPrefix(q.Data, "pack_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "pack_"))
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", fmt.Sprintf("edit_%d", id)),
				tgbotapi.NewInlineKeyboardButtonData("🗑️ Удалить", fmt.Sprintf("del_%d", id)),
			))
		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Что сделать со скидкой?")
		msg.ReplyMarkup = mk
		if _, err := h.sender.Send(ctx, msg); err != nil {
			log.Println(err)
		}

	case strings.HasPrefix(q.Data, "del_"):
		id := strings.TrimPrefix(q.Data, "del_")
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", "delok_"+id),
			))
		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Точно удалить?")
		msg.ReplyMarkup = mk
		if _, err := h.sender.Send(ctx, msg); err != nil {
			log.Println(err)
		}

	case strings.HasPrefix(q.Data, "delok_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "delok_"))
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if err := h.service.Repo.DeleteStickerPack(dbctx, id); err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "Ошибка удаления: "+err.Error()))
		} else {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "✅ Удалено"))
		}

	case strings.HasPrefix(q.Data, "edit_"):
		id := strings.TrimPrefix(q.Data, "edit_")
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: q.From.ID, State: "edit_wait_name", Data: id,
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "Отправьте новое название:"))
	}
}

func (h *Handler) handleAdminCommand(ctx context.Context, m *tgbotapi.Message) {
	switch m.Command() {
	case "start":
		h.sendStartMessage(ctx, m.Chat.ID)
	case "packs":
		h.showPacksList(ctx, m.Chat.ID)
	case "addpack":
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_name",
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Отправьте название новой скидки:"))
	case "draw":
		h.processDraw(ctx, m.Chat.ID, m.From.ID)
	}
}

func (h *Handler) showPacksList(ctx context.Context, chatID int64) {
	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	packs, err := h.service.Repo.GetStickerPacks(dbctx)
	if err != nil {
		log.Println("GetStickerPacks:", err)
		return
	}
	if len(packs) == 0 {
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Скидок не добавлено"))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range packs {
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("[%d] %s", p.ID, p.Name), fmt.Sprintf("pack_%d", p.ID))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	mk := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, "Выберите скидку:")
	msg.ReplyMarkup = mk
	_, _ = h.sender.Send(ctx, msg)
}

func (h *Handler) handleAdminDialog(ctx context.Context, m *tgbotapi.Message) {
	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	st, _ := h.service.Repo.GetAdminState(dbctx, m.From.ID)

	switch st.State {

	case "add_wait_name":
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_url", Data: m.Text,
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте значение скидки:"))

	case "add_wait_url":
		if err := h.service.Repo.CreateStickerPack(dbctx, st.Data, m.Text); err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			return
		}
		_ = h.service.Repo.ClearAdminState(dbctx, m.From.ID)
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "✅ Скидка добавлена"))

	case "edit_wait_name":
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "edit_wait_url", Data: st.Data + "|" + m.Text,
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте новое значение скидки:"))

	case "edit_wait_url":
		parts := strings.SplitN(st.Data, "|", 2)
		id, _ := strconv.Atoi(parts[0])
		newName := parts[1]
		newURL := m.Text
		if err := h.service.Repo.UpdateStickerPack(dbctx, id, newName, newURL); err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			return
		}
		_ = h.service.Repo.ClearAdminState(dbctx, m.From.ID)
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "✅ Обновлено"))
	}
}

func (h *Handler) subscribed(ctx context.Context, userID int64) bool {
	if h.subChannelID == 0 {
		return true
	}
	// Учитываем общий лимит Telegram
	if err := h.sender.Wait(ctx); err != nil {
		log.Println("rate wait:", err)
		return false
	}

	cfg := tgbotapi.ChatConfigWithUser{ChatID: h.subChannelID, UserID: userID}
	member, err := h.bot.GetChatMember(tgbotapi.GetChatMemberConfig{ChatConfigWithUser: cfg})
	if err != nil {
		log.Println("GetChatMember:", err)
		return false
	}
	switch member.Status {
	case "creator", "administrator", "member":
		return true
	default:
		return false
	}
}

func (h *Handler) processDraw(ctx context.Context, chatID, userID int64) {
	// Проверка подписки
	subCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if !h.subscribed(subCtx, userID) {
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Проверить подписку", "draw"),
			))
		msg := tgbotapi.NewMessage(chatID, "Подпишись на канал "+h.subChannelLink+", чтобы получить скидку")
		msg.ReplyMarkup = mk
		_, _ = h.sender.Send(ctx, msg)
		return
	}

	// Клейм + выбор пакета
	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	p, err := h.service.ClaimStickerPack(dbctx, userID, h.adminID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAlreadyClaimed):
			mk := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("Забронировать столик", h.shopURL),
				))
			msg := tgbotapi.NewMessage(chatID,
				"⚡️<u>Попытка была одна — и Фортуна уже подарила тебе особую скидку!</u>\n\n"+
					"Забронируй столик на нашем сайте и воспользуйся скидкой в ресторане:\n"+
					"🔹<a href=\"https://ketino.ru\">НАШ САЙТ</a>\n"+
					"🔸<a href=\"https://instagram.com/ketino_rest\">INSTA</a>\n"+
					"🔹<a href=\"https://vk.com/ketinorest\">VKONTAKTE</a>\n"+
					"🔸<a href=\"https://t.me/ketinorest\">TELEGRAM</a>\n")
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = mk
			_, _ = h.sender.Send(ctx, msg)
			return
		case errors.Is(err, repositories.ErrNoPacks):
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "⚠️ Скидок пока нет. Попробуйте позже."))
			return
		default:
			log.Println("ClaimStickerPack:", err)
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Произошла ошибка. Попробуйте позже."))
			return
		}
	}

	// Отправляем "кубик" сразу…
	dice := tgbotapi.NewDice(chatID)
	dice.Emoji = "🎲"
	_, _ = h.sender.Send(ctx, dice)

	// …а дальше — без блокировки текущего воркера
	go func(chatID int64, url, shop string) {
		time.Sleep(2 * time.Second)

		text := "Ваша счастливая скидка:\n" +
			"👉<u><b>" + url + "</b></u>👈"
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = tgbotapi.ModeHTML
		_, _ = h.sender.Send(context.Background(), msg)

		time.Sleep(1 * time.Second)

		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Забронировать столик", shop),
			))
		after := "⚡️<u>Попытка была одна — и Фортуна уже подарила тебе особую скидку!</u>\n\n" +
			"Забронируй столик на нашем сайте и воспользуйся скидкой в ресторане:\n" +
			"🔹<a href=\"https://ketino.ru\">НАШ САЙТ</a>\n" +
			"🔸<a href=\"https://instagram.com/ketino_rest\">INSTA</a>\n" +
			"🔹<a href=\"https://vk.com/ketinorest\">VKONTAKTE</a>\n" +
			"🔸<a href=\"https://t.me/ketinorest\">TELEGRAM</a>\n"

		am := tgbotapi.NewMessage(chatID, after)
		am.ParseMode = tgbotapi.ModeHTML
		am.ReplyMarkup = mk
		_, _ = h.sender.Send(context.Background(), am)
	}(chatID, p.URL, h.shopURL)
}
