package logic

import (
	"github.com/zhangpanyi/basebot/logger"
	"github.com/zhangpanyi/basebot/telegram/methods"
	"github.com/zhangpanyi/basebot/telegram/types"
	"luckybot/app/logic/context"
	"luckybot/app/logic/handlers"
	"luckybot/app/storage/models"
)

// NewUpdate 机器人更新
func NewUpdate(bot *methods.BotExt, update *types.Update) {
	// 展示红包
	if update.InlineQuery != nil {
		handlers.ShowLuckyMoney(bot, update.InlineQuery)
		return
	}

	// 获取用户ID
	var fromID int64
	if update.Message != nil {
		fromID = update.Message.From.ID
		if update.Message.Chat.Type != types.ChatPrivate {
			return
		}

		// 添加订户
		model := models.SubscriberModel{}
		_ = model.AddSubscriber(fromID)
	} else if update.CallbackQuery != nil {
		fromID = update.CallbackQuery.From.ID
	} else {
		return
	}

	// 获取操作记录
	r, err := context.GetRecord(uint32(fromID))
	if err != nil {
		logger.Warnf("Failed to get bot record, bot_id: %v, %v, %v", bot.ID, fromID, err)
		return
	}

	// 领取红包
	if update.CallbackQuery != nil && update.CallbackQuery.InlineMessageID != nil {
		new(handlers.ReceiveHandler).Handle(bot, r, update)
		return
	}

	// 处理机器人请求
	new(handlers.MainMenuHandler).Handle(bot, r, update)

	// 删除空操作记录
	if r.Empty() {
		context.DelRecord(uint32(fromID))
	}
}
