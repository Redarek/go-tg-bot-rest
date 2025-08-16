package handlers

import (
	"context"
	_ "embed"
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
	service        *services.Service
	adminID        int64
	shopURL        string
	subChannelID   int64
	subChannelLink string
}

func NewHandler(bot *tgbotapi.BotAPI, db *pgxpool.Pool, cfg *config.Config) *Handler {
	repo := repositories.NewRepository(db)
	return &Handler{
		bot:            bot,
		service:        services.NewService(repo),
		adminID:        cfg.AdminID,
		shopURL:        cfg.ShopURL,
		subChannelID:   cfg.SubChannelID,
		subChannelLink: cfg.SubChannelLink,
	}
}

func (h *Handler) HandleUpdate(upd tgbotapi.Update) {
	ctx := context.Background()

	if upd.Message != nil {

		if upd.Message.IsCommand() && upd.Message.From.ID == h.adminID {
			h.handleAdminCommand(ctx, upd.Message)
			return
		}

		if upd.Message.IsCommand() &&
			upd.Message.From.ID != h.adminID &&
			upd.Message.Command() == "draw" {

			h.processDraw(ctx, upd.Message.Chat.ID, upd.Message.From.ID)
			return
		}

		if upd.Message.IsCommand() &&
			upd.Message.From.ID != h.adminID &&
			upd.Message.Command() == "start" {
			h.sendStartMessage(upd.Message.Chat.ID)
			return
		}

		h.handleAdminDialog(ctx, upd.Message)
		return
	}

	if upd.CallbackQuery != nil {
		h.handleCallback(ctx, upd.CallbackQuery)
	}
}

func (h *Handler) sendStartMessage(chatID int64) {
	err := h.service.Repo.UpsertBotUser(context.Background(), chatID)
	if err != nil {
		return // todo
	}

	mk := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Получить скидку", "draw"),
		))

	caption := "🎯<b><u>Готов испытать удачу?</u></b>\n" +
		"Запускай наше «Колесо Вкуса» и забирай случайную скидку на заказ в нашем ресторане!</b>\n" +
		"☸️<i>Крути колесо и приходи за своим вкусным бонусом!</i>"

	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
		Name:  "start.jpg",
		Bytes: StartJPG,
	})
	photo.Caption = caption
	photo.ReplyMarkup = mk
	photo.ParseMode = tgbotapi.ModeHTML

	if _, err := h.bot.Send(photo); err != nil {
		_ = err
	}
}

func (h *Handler) handleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	switch {
	case q.Data == "start":
		h.sendStartMessage(q.Message.Chat.ID)

	case q.Data == "draw":
		h.processDraw(ctx, q.Message.Chat.ID, q.From.ID)

	case strings.HasPrefix(q.Data, "promotion_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "promotion_"))
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", fmt.Sprintf("edit_%d", id)),
				tgbotapi.NewInlineKeyboardButtonData("🗑️ Удалить", fmt.Sprintf("del_%d", id)),
			))
		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Что сделать со скидкой?")
		msg.ReplyMarkup = mk
		_, err := h.bot.Send(msg)
		if err != nil {
			return // todo
		}

	case strings.HasPrefix(q.Data, "del_"):
		id := strings.TrimPrefix(q.Data, "del_")
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", "delok_"+id),
			))

		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Точно удалить?")
		msg.ReplyMarkup = mk
		_, err := h.bot.Send(msg)
		if err != nil {
			return // todo
		}

	case strings.HasPrefix(q.Data, "delok_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "delok_"))
		if err := h.service.Repo.DeletePromotion(ctx, id); err != nil {
			_, err = h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID,
				"Ошибка удаления: "+err.Error()))
			if err != nil {
				return // todo
			}
		} else {
			_, err = h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID, "✅ Удалено"))
			if err != nil {
				return // todo
			}
		}

	case strings.HasPrefix(q.Data, "edit_"):
		id := strings.TrimPrefix(q.Data, "edit_")
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: q.From.ID, State: "edit_wait_name", Data: id,
		})
		_, err := h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID,
			"Отправьте новое название:"))
		if err != nil {
			return // todo
		}
	}
}

func (h *Handler) handleAdminCommand(ctx context.Context, m *tgbotapi.Message) {
	switch m.Command() {
	case "start":
		h.sendStartMessage(m.Chat.ID)

	case "promotions":
		h.showPromotionsList(ctx, m.Chat.ID)

	case "addpromotion":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_name",
		})
		_, err := h.bot.Send(tgbotapi.NewMessage(m.Chat.ID,
			"Отправьте название новой скидки:"))
		if err != nil {
			return // todo
		}

	case "draw":
		h.processDraw(ctx, m.Chat.ID, m.From.ID)
	}
}

func (h *Handler) showPromotionsList(ctx context.Context, chatID int64) {
	promotions, _ := h.service.Repo.GetPromotions(ctx)
	if len(promotions) == 0 {
		_, err := h.bot.Send(tgbotapi.NewMessage(chatID, "Скидок не добавлено"))
		if err != nil {
			return // todo
		}
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range promotions {
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("[%d] %s", p.ID, p.Name),
			fmt.Sprintf("promotion_%d", p.ID))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	mk := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, "Выберите скидку:")
	msg.ReplyMarkup = mk
	_, err := h.bot.Send(msg)
	if err != nil {
		return // todo
	}
}

func (h *Handler) handleAdminDialog(ctx context.Context, m *tgbotapi.Message) {
	st, _ := h.service.Repo.GetAdminState(ctx, m.From.ID)

	switch st.State {

	case "add_wait_name":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_url", Data: m.Text,
		})
		_, err := h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте ссылку:"))
		if err != nil {
			return // todo
		}

	case "add_wait_url":
		if err := h.service.Repo.CreatePromotion(ctx, st.Data, m.Text); err != nil {
			_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			if err != nil {
				return // todo
			}
			return
		}
		err := h.service.Repo.ClearAdminState(ctx, m.From.ID)
		if err != nil {
			return // todo
		}
		_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Скидка добавлена"))
		if err != nil {
			return // todo
		}

	case "edit_wait_name":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID,
			State:  "edit_wait_url",
			Data:   st.Data + "|" + m.Text,
		})
		_, err := h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте новую ссылку:"))
		if err != nil {
			return // todo
		}

	case "edit_wait_url":
		parts := strings.SplitN(st.Data, "|", 2)
		id, _ := strconv.Atoi(parts[0])
		newName := parts[1]
		newURL := m.Text
		if err := h.service.Repo.UpdatePromotion(ctx, id, newName, newURL); err != nil {
			_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			if err != nil {
				return // todo
			}
			return
		}
		err := h.service.Repo.ClearAdminState(ctx, m.From.ID)
		if err != nil {
			return // todo
		}
		_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Обновлено"))
		if err != nil {
			return // todo
		}
	}
}

func (h *Handler) subscribed(userID int64) bool {
	if h.subChannelID == 0 {
		return true
	}

	cfg := tgbotapi.ChatConfigWithUser{
		ChatID: h.subChannelID,
		UserID: userID,
	}

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
	if !h.subscribed(userID) {
		_, err := h.bot.Send(tgbotapi.NewMessage(chatID,
			"Сначала нужно подписаться на канал "+h.subChannelLink))
		if err != nil {
			return // todo
		}
		return
	}

	p, err := h.service.ClaimPromotion(ctx, userID, h.adminID)
	if err != nil {
		if strings.Contains(err.Error(), "Список скидок пуст") {
			_, err = h.bot.Send(tgbotapi.NewMessage(chatID,
				"⚠️ Скидок пока нет. Попробуйте позже 🕒"))
			if err != nil {
				return // todo
			}
			return
		}
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Забронировать столик", h.shopURL),
			))

		msg := tgbotapi.NewMessage(chatID, err.Error())
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = mk
		_, err = h.bot.Send(msg)
		if err != nil {
			return // todo
		}
		return
	}

	dice := tgbotapi.NewDice(chatID)
	dice.Emoji = "🎲" // есть ещё 🎲 ⚽ 🏀 🎳 🎯🎰
	_, err = h.bot.Send(dice)
	if err != nil {
		return // todo
	}

	time.Sleep(2 * time.Second)

	text := "😋<b>ВОТ ЭТО НАХОДКА!</b> Ты получил свою вкусную скидку!\n" +
		"🍷Теперь осталось только прийти, заказать любимые блюда и насладиться вечером.\n" + p.URL

	res := tgbotapi.NewMessage(chatID, text)
	res.ParseMode = tgbotapi.ModeHTML
	_, err = h.bot.Send(res)
	if err != nil {
		return // todo
	}

	time.Sleep(1 * time.Second)
	textAfterDraw := "⚡️<u>Попытка была одна — и Фортуна уже подарила тебе особую скидку!</u>\n" +
		"Забронируй столик на нашем сайте и воспользуйся скидкой в ресторане:\n" +
		"🟣<a href=\"https://ketino.ru\">Наш сайт</a>\n" +
		"🔵<a href=\"https://instagram.com/ketino_rest\">Инста</a>\n" +
		"🟣<a href=\"https://vk.com/ketinorest\">ВКонтакте</a>"

	resAfterDraw := tgbotapi.NewMessage(chatID, textAfterDraw)
	resAfterDraw.ParseMode = tgbotapi.ModeHTML
	_, err = h.bot.Send(resAfterDraw)
	if err != nil {
		return // todo
	}
}
