package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/media"
)

func UserMenu() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔗 Havola yuborish"),
			tgbotapi.NewKeyboardButton("📁 Saqlanganlar"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("👤 Profil"),
			tgbotapi.NewKeyboardButton("💰 Donat"),
			tgbotapi.NewKeyboardButton("ℹ️ Qo'llanma"),
		),
	)
}

func SelectionKeyboard(token string, variantType media.VariantType, quality media.Quality) tgbotapi.InlineKeyboardMarkup {
	video := "🎬 Video"
	audio := "🎵 Audio MP3"
	if variantType == media.VariantVideo {
		video += " ✅"
	}
	if variantType == media.VariantAudio {
		audio += " ✅"
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			cb(video, "sel:"+token+":type:video"),
			cb(audio, "sel:"+token+":type:audio"),
		),
		tgbotapi.NewInlineKeyboardRow(
			cb(mark("⚡ Auto", quality == media.QualityAuto), "sel:"+token+":q:AUTO"),
			cb(mark("🎞 Original", quality == media.QualityOriginal), "sel:"+token+":q:ORIGINAL"),
		),
		tgbotapi.NewInlineKeyboardRow(
			cb(mark("📺 1080p", quality == media.QualityP1080), "sel:"+token+":q:P1080"),
			cb(mark("📱 720p", quality == media.QualityP720), "sel:"+token+":q:P720"),
		),
		tgbotapi.NewInlineKeyboardRow(
			cb(mark("📉 480p", quality == media.QualityP480), "sel:"+token+":q:P480"),
			cb(mark("📦 Kichik hajm", quality == media.QualitySmall), "sel:"+token+":q:SMALL"),
		),
		tgbotapi.NewInlineKeyboardRow(
			cb("✅ Yuklash", "sel:"+token+":download"),
			cb("❌ Bekor qilish", "sel:"+token+":cancel"),
		),
	)
}

func MediaActionsKeyboard(variantID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			cb("🎵 MP3", fmt.Sprintf("media:%d:mp3", variantID)),
			cb("📄 Ma'lumot", fmt.Sprintf("media:%d:info", variantID)),
			cb("📁 Saqlash", fmt.Sprintf("media:%d:save", variantID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			cb("♻️ Boshqa sifat", fmt.Sprintf("media:%d:quality", variantID)),
			cb("📤 Ulashish", fmt.Sprintf("media:%d:share", variantID)),
			cb("🗑 O'chirish", fmt.Sprintf("media:%d:delete", variantID)),
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
		row = append(row, cb("Keyingi ➡️", fmt.Sprintf("saved:%d", page+1)))
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func AdminKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cb("📊 Statistika", "admin:stats"), cb("👥 Userlar", "admin:users")),
		tgbotapi.NewInlineKeyboardRow(cb("📥 Downloadlar", "admin:downloads"), cb("💰 Donatlar", "admin:donate")),
		tgbotapi.NewInlineKeyboardRow(cb("👮 Adminlar", "admin:admins"), cb("⚙️ Bot sozlamalari", "admin:settings")),
		tgbotapi.NewInlineKeyboardRow(cb("🧾 Loglar", "admin:logs")),
	)
}

func AdminSettingsKeyboard(online, maintenance bool) tgbotapi.InlineKeyboardMarkup {
	onlineText := "🟢 Online"
	if online {
		onlineText += " ✅"
	}
	offlineText := "🔴 Offline"
	if !online {
		offlineText += " ✅"
	}
	maintenanceText := "🛠 Maintenance OFF"
	if maintenance {
		maintenanceText = "🛠 Maintenance ON ✅"
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cb(onlineText, "admin:set:online"), cb(offlineText, "admin:set:offline")),
		tgbotapi.NewInlineKeyboardRow(cb(maintenanceText, "admin:set:maintenance")),
		tgbotapi.NewInlineKeyboardRow(cb("📦 Max video hajm", "admin:max:video"), cb("🎵 Max audio hajm", "admin:max:audio")),
		tgbotapi.NewInlineKeyboardRow(cb("💳 Donat karta", "admin:edit:donate_card_number"), cb("📝 Welcome text", "admin:edit:welcome_text")),
		tgbotapi.NewInlineKeyboardRow(cb("📝 Help text", "admin:edit:help_text"), cb("⬅️ Orqaga", "admin:home")),
	)
}

func LimitPresetKeyboard(kind string, values []int64) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(values)/2+2)
	for i := 0; i < len(values); i += 2 {
		row := []tgbotapi.InlineKeyboardButton{cb(fmt.Sprintf("%d MB", values[i]), fmt.Sprintf("admin:limit:%s:%d", kind, values[i]))}
		if i+1 < len(values) {
			row = append(row, cb(fmt.Sprintf("%d MB", values[i+1]), fmt.Sprintf("admin:limit:%s:%d", kind, values[i+1])))
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

func mark(text string, selected bool) string {
	if selected {
		return text + " ✅"
	}
	return text
}
