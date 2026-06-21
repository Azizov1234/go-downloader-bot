package admin

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) showUsers(ctx context.Context, chatID int64, messageID int, page int) {
	limit := 10
	users, total, err := h.users.List(ctx, page, limit)
	if err != nil {
		h.edit(chatID, messageID, "Foydalanuvchilarni yuklashda xatolik yuz berdi.", tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Orqaga", "admin:home")),
		))
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("👥 Foydalanuvchilar ro'yxati (Jami: %d)\n\n", total))

	if len(users) == 0 {
		sb.WriteString("Hozircha foydalanuvchilar mavjud emas.")
	} else {
		for i, u := range users {
			num := (page-1)*limit + i + 1
			usernameStr := "yo'q"
			if u.Username != "" {
				usernameStr = "@" + u.Username
			}
			statusEmoji := "🟢"
			if u.Status == "BLOCKED" {
				statusEmoji = "🔴"
			}
			fmt.Fprintf(&sb, "%d. %s ID: %d | %s | %s\n", num, statusEmoji, u.ID, u.FullName, usernameStr)
		}
	}

	fmt.Fprintf(&sb, "\nSahifa: %d / %d", page, totalPages)

	var rows [][]tgbotapi.InlineKeyboardButton
	var idRow []tgbotapi.InlineKeyboardButton
	for _, u := range users {
		idRow = append(idRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("#%d", u.ID), fmt.Sprintf("admin:user:view:%d", u.ID)))
		if len(idRow) == 5 {
			rows = append(rows, idRow)
			idRow = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(idRow) > 0 {
		rows = append(rows, idRow)
	}

	var navRow []tgbotapi.InlineKeyboardButton
	if page > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Oldingi", fmt.Sprintf("admin:users:%d", page-1)))
	}
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️ Keyingi", fmt.Sprintf("admin:users:%d", page+1)))
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔍 Qidirish", "admin:user:search"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Orqaga", "admin:home"),
	))

	h.edit(chatID, messageID, sb.String(), tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) showUserProfile(ctx context.Context, chatID int64, messageID int, id int64) {
	p, err := h.users.ProfileByUserID(ctx, id)
	if err != nil {
		h.edit(chatID, messageID, "Foydalanuvchi ma'lumotlarini yuklashda xatolik yuz berdi.", tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Orqaga", "admin:users:1")),
		))
		return
	}

	usernameStr := "yo'q"
	if p.Username != "" {
		usernameStr = "@" + p.Username
	}
	statusEmoji := "🟢 Faol (ACTIVE)"
	if p.Status == "BLOCKED" {
		statusEmoji = "🔴 Bloklangan (BLOCKED)"
	}
	lastActive := "hali faollik yo'q"
	if p.LastDownloadAt != nil {
		lastActive = p.LastDownloadAt.Format("2006-01-02 15:04:05")
	}

	text := fmt.Sprintf(
		"👤 Foydalanuvchi ma'lumotlari:\n\nTizim ID: %d\nTelegram ID: %d\nUsername: %s\nIsm: %s\nHolati: %s\n\nYuklamalar soni: %d (Muvaffaqiyatli: %d, Xato: %d)\nBugungi yuklamalar: %d\nSaqlanganlar: %d\nOxirgi faollik: %s",
		p.ID, p.TelegramID, usernameStr, p.FullName, statusEmoji,
		p.DownloadsCount, p.SuccessDownloads, p.FailedDownloads, p.TodayDownloads, p.SavedCount, lastActive,
	)

	var rows [][]tgbotapi.InlineKeyboardButton
	if p.Status == "ACTIVE" {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚫 Bloklash (Nofaol qilish)", fmt.Sprintf("admin:user:status:%d:BLOCKED", p.ID)),
		))
	} else {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟢 Faollashtirish (Blokdan chiqarish)", fmt.Sprintf("admin:user:status:%d:ACTIVE", p.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Orqaga", "admin:users:1"),
	))

	h.edit(chatID, messageID, text, tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) searchUsers(ctx context.Context, chatID int64, query string) {
	if query == "" {
		h.send(chatID, "Qidiruv so'rovi bo'sh bo'lishi mumkin emas.", nil)
		return
	}
	users, err := h.users.Search(ctx, query, 10)
	if err != nil {
		h.send(chatID, "Qidiruvda xatolik yuz berdi.", nil)
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Qidiruv natijalari: %q uchun\n\n", query))

	if len(users) == 0 {
		sb.WriteString("Hech qanday foydalanuvchi topilmadi.")
	} else {
		for i, u := range users {
			usernameStr := "yo'q"
			if u.Username != "" {
				usernameStr = "@" + u.Username
			}
			statusEmoji := "🟢"
			if u.Status == "BLOCKED" {
				statusEmoji = "🔴"
			}
			fmt.Fprintf(&sb, "%d. %s ID: %d | %s | %s\n", i+1, statusEmoji, u.ID, u.FullName, usernameStr)
		}
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var idRow []tgbotapi.InlineKeyboardButton
	for _, u := range users {
		idRow = append(idRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("#%d", u.ID), fmt.Sprintf("admin:user:view:%d", u.ID)))
		if len(idRow) == 5 {
			rows = append(rows, idRow)
			idRow = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(idRow) > 0 {
		rows = append(rows, idRow)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Userlar ro'yxatiga", "admin:users:1"),
	))

	h.send(chatID, sb.String(), tgbotapi.NewInlineKeyboardMarkup(rows...))
}
