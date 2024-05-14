package pusher

import (
	"github.com/zhangpanyi/basebot/telegram/methods"
	"luckybot/app/logic/botext"
)

// Post 投递消息
func Post(receiver int64, text string, markdownMode bool,
	markup *methods.InlineKeyboardMarkup) {
	if gpusher != nil && botext.GetBot() != nil {
		gpusher.push(botext.GetBot(), receiver, text, markdownMode, markup)
	}
}
