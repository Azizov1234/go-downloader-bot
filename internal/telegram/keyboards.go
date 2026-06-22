package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func UserMenu() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔗 Havola yuborish"),
			tgbotapi.NewKeyboardButton("📂 Saqlanganlar"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("👤 Profil"),
			tgbotapi.NewKeyboardButton("💳 Donat"),
			tgbotapi.NewKeyboardButton("ℹ️ Qo'llanma"),
		),
	)
}


func MediaActionsKeyboard(variantID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			cb("🎵 MP3", fmt.Sprintf("media:%d:mp3", variantID)),
			cb("ℹ️ Ma'lumot", fmt.Sprintf("media:%d:info", variantID)),
			cb("💾 Saqlash", fmt.Sprintf("media:%d:save", variantID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			cb("📤 Ulashish", fmt.Sprintf("media:%d:share", variantID)),
			cb("🗑️ O'chirish", fmt.Sprintf("media:%d:delete", variantID)),
		),
	)
}

func SavedPaginationKeyboard(page, total, perPage int) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{}
	row := []tgbotapi.InlineKeyboardButton{}
	if page > 1 {
		row = append(row, cb("⬅️ Oldingi", fmt.Sprintf("saved:%d", page-1)))
	}
	if page*perPage < total {
		row = append(row, cb("➡️ Keyingi", fmt.Sprintf("saved:%d", page+1)))
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func AdminKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cb("📊 Statistika", "admin:stats"), cb("👥 Userlar", "admin:users")),
		tgbotapi.NewInlineKeyboardRow(cb("📥 Downloadlar", "admin:downloads"), cb("💳 Donatlar", "admin:donate")),
		tgbotapi.NewInlineKeyboardRow(cb("🛡️ Adminlar", "admin:admins"), cb("⚙️ Bot sozlamalari", "admin:settings")),
		tgbotapi.NewInlineKeyboardRow(cb("📜 Loglar", "admin:logs")),
	)
}

func AdminSettingsKeyboard(online, maintenance bool, apiMode string) tgbotapi.InlineKeyboardMarkup {
	onlineText := "🟢 Online"
	if online {
		onlineText += " ☑️"
	} else {
		onlineText += " ⬜"
	}
	offlineText := "🔴 Offline"
	if !online {
		offlineText += " ☑️"
	} else {
		offlineText += " ⬜"
	}
	maintenanceText := "🔧 Texnik ishlar: OFF ⬜"
	if maintenance {
		maintenanceText = "⚠️ Texnik ishlar: ON ☑️"
	}
	cloudText := "☁️ Cloud mode"
	localText := "💻 Local mode"
	if apiMode == "local" {
		localText += " ☑️"
		cloudText += " ⬜"
	} else {
		cloudText += " ☑️"
		localText += " ⬜"
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cb(onlineText, "admin:set:online"), cb(offlineText, "admin:set:offline")),
		tgbotapi.NewInlineKeyboardRow(cb(maintenanceText, "admin:set:maintenance")),
		tgbotapi.NewInlineKeyboardRow(cb("🔌 Telegram API mode", "admin:telegram:health")),
		tgbotapi.NewInlineKeyboardRow(cb(cloudText, "admin:telegram:cloud"), cb(localText, "admin:telegram:local")),
		tgbotapi.NewInlineKeyboardRow(cb("📤 Cloud upload limit", "admin:max:cloud_upload"), cb("📥 Local upload limit", "admin:max:local_upload")),
		tgbotapi.NewInlineKeyboardRow(cb("🩺 Local API holati", "admin:telegram:health")),
		tgbotapi.NewInlineKeyboardRow(cb("📹 Max video hajm", "admin:max:video"), cb("🎵 Max audio hajm", "admin:max:audio")),
		tgbotapi.NewInlineKeyboardRow(cb("💳 Donat karta", "admin:edit:donate_card_number"), cb("📝 Welcome text", "admin:edit:welcome_text")),
		tgbotapi.NewInlineKeyboardRow(cb("📖 Help text", "admin:edit:help_text"), cb("⬅️ Orqaga", "admin:home")),
	)
}

func LimitPresetKeyboard(kind string, values []int64) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(values)/2+2)
	for i := 0; i < len(values); i += 2 {
		row := []tgbotapi.InlineKeyboardButton{cb(fmt.Sprintf("💾 %d MB", values[i]), fmt.Sprintf("admin:limit:%s:%d", kind, values[i]))}
		if i+1 < len(values) {
			row = append(row, cb(fmt.Sprintf("💾 %d MB", values[i+1]), fmt.Sprintf("admin:limit:%s:%d", kind, values[i+1])))
		}
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(cb("✍️ Qo'lda kiritish", "admin:limit:"+kind+":manual")))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(cb("⬅️ Orqaga", "admin:settings")))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func cb(text, data string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(text, data)
}
