package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Redarek/go-tg-bot-rest/pkg/config"
	"github.com/Redarek/go-tg-bot-rest/pkg/db"
	"github.com/Redarek/go-tg-bot-rest/pkg/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.LoadConfig()
	if cfg.TelegramToken == "" {
		log.Fatal("TELEGRAM_APITOKEN не задан")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("Telegram init error: %v", err)
	}
	log.Printf("Authorized as @%s", bot.Self.UserName)

	pub := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Начать работу"},
		tgbotapi.BotCommand{Command: "draw", Description: "Получить скидку"},
	)
	publicScope := tgbotapi.NewBotCommandScopeDefault()
	pub.Scope = &publicScope
	_, _ = bot.Request(pub)

	admin := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Начать работу"},
		tgbotapi.BotCommand{Command: "promotions", Description: "Список скидок"},
		tgbotapi.BotCommand{Command: "addpromotion", Description: "Добавить скидку"},
	)
	adminScope := tgbotapi.NewBotCommandScopeChat(cfg.AdminID)
	admin.Scope = &adminScope
	_, _ = bot.Request(admin)

	pool := db.Connect(cfg)
	defer pool.Close()

	h := handlers.NewHandler(bot, pool, cfg)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Bot started")
	for {
		select {
		case <-ctx.Done():
			return
		case upd := <-updates:
			h.HandleUpdate(upd)
		}
	}
}
