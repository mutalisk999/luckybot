package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/zhangpanyi/basebot/history"
	"github.com/zhangpanyi/basebot/logger"
	"github.com/zhangpanyi/basebot/telegram/methods"
	"github.com/zhangpanyi/basebot/telegram/types"
	"luckybot/app/location"
	"luckybot/app/logic/handlers/utils"
	"luckybot/app/storage/models"
)

// PageLimit 每页条目
const PageLimit = 5

// 匹配历史页数
var reMathHistoryPage *regexp.Regexp

func init() {
	var err error
	reMathHistoryPage, err = regexp.Compile("^/history/(|(\\d+)/)$")
	if err != nil {
		panic(err)
	}
}

// HistoryHandler 历史记录
type HistoryHandler struct {
}

// Handle 消息处理
func (handler *HistoryHandler) Handle(bot *methods.BotExt, r *history.History, update *types.Update) {
	data := update.CallbackQuery.Data
	result := reMathHistoryPage.FindStringSubmatch(data)
	if len(result) == 3 {
		page, err := strconv.Atoi(result[2])
		if err != nil {
			handler.replyHistory(bot, 1, update.CallbackQuery)
		} else {
			handler.replyHistory(bot, page, update.CallbackQuery)
		}
	}
}

// 消息路由
func (handler *HistoryHandler) route(bot *methods.BotExt, query *types.CallbackQuery) Handler {
	return nil
}

// 生成菜单列表
func (handler *HistoryHandler) makeMenuList(fromID int64, page, pagesum int) *methods.InlineKeyboardMarkup {
	privpage := page - 1
	if privpage < 1 {
		privpage = 1
	}
	nextpage := page + 1
	if nextpage > pagesum {
		nextpage = pagesum
	}
	priv := fmt.Sprintf("/history/%d/", privpage)
	next := fmt.Sprintf("/history/%d/", nextpage)
	menus := [...]methods.InlineKeyboardButton{
		methods.InlineKeyboardButton{Text: tr(fromID, "lng_previous_page"), CallbackData: priv},
		methods.InlineKeyboardButton{Text: tr(fromID, "lng_next_page"), CallbackData: next},
		methods.InlineKeyboardButton{Text: tr(fromID, "lng_back_superior"), CallbackData: "/main/"},
	}
	return methods.MakeInlineKeyboardMarkupAuto(menus[:], 2)
}

// 生成回复内容
func (handler *HistoryHandler) makeReplyContent(fromID int64, array []*models.Version, page, pagesum uint) string {
	header := fmt.Sprintf("%s (*%d*/%d)\n\n", tr(fromID, "lng_history"), page, pagesum)
	if len(array) > 0 {
		lines := make([]string, 0, len(array))
		for _, version := range array {
			date := location.Format(version.Timestamp)
			lines = append(lines, fmt.Sprintf("`%s` %s", date, utils.MakeHistoryMessage(fromID, version)))
		}
		return header + strings.Join(lines, "\n\n")
	}
	return header + tr(fromID, "lng_priv_history_no_op")
}

// 回复历史记录
func (handler *HistoryHandler) replyHistory(bot *methods.BotExt, page int, query *types.CallbackQuery) {
	// 检查页数
	if page < 1 {
		page = 1
	}

	// 查询历史
	fromID := query.From.ID
	model := models.AccountVersionModel{}
	history, sum, err := model.GetVersions(fromID, uint((page-1)*PageLimit), PageLimit, true)
	if err != nil {
		logger.Warnf("Failed to query user history, %v", err)
	}
	pagesum := sum / PageLimit
	if sum%PageLimit > 0 {
		pagesum++
	}

	// 回复内容
	if len(history) > 0 {
		_ = bot.AnswerCallbackQuery(query, "", false, "", 0)
	} else {
		reply := tr(fromID, "lng_history_no_op")
		_ = bot.AnswerCallbackQuery(query, reply, false, "", 0)
		return
	}
	reply := handler.makeReplyContent(fromID, history, uint(page), uint(pagesum))
	_, _ = bot.EditMessageReplyMarkup(query.Message, reply, true, handler.makeMenuList(fromID, page, pagesum))
}
