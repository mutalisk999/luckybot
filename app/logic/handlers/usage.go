package handlers

import (
	"fmt"

	"github.com/zhangpanyi/basebot/history"
	"github.com/zhangpanyi/basebot/telegram/methods"
	"github.com/zhangpanyi/basebot/telegram/types"
	"luckybot/app"
	"luckybot/app/config"
)

// UsageHandler 使用说明
type UsageHandler struct {
}

// Handle 消息处理
func (*UsageHandler) Handle(bot *methods.BotExt, r *history.History, update *types.Update) {
	fromID := update.CallbackQuery.From.ID
	menus := [...]methods.InlineKeyboardButton{
		methods.InlineKeyboardButton{
			Text:         tr(fromID, "lng_back_superior"),
			CallbackData: "/main/",
		},
	}
	markup := methods.MakeInlineKeyboardMarkupAuto(menus[:], 1)

	var supportStaff int64
	serveCfg := config.GetServe()
	if serveCfg.SupportStaff != nil {
		supportStaff = *serveCfg.SupportStaff
	}

	reply := tr(fromID, "lng_usage_say")
	version := fmt.Sprintf("Version: %s", app.VERSION)
	github := fmt.Sprintf("Fork from Github: [%s](%s)", app.GITHUB, app.GITHUB)
	reply = fmt.Sprintf("%s\n\n%s\n%s", fmt.Sprintf(reply, serveCfg.Name, supportStaff), version, github)

	_ = bot.AnswerCallbackQuery(update.CallbackQuery, "", false, "", 0)
	_, _ = bot.EditMessageReplyMarkupDisableWebPagePreview(update.CallbackQuery.Message, reply, true, markup)
}

// 消息路由
func (*UsageHandler) route(bot *methods.BotExt, query *types.CallbackQuery) Handler {
	return nil
}
