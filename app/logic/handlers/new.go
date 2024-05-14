package handlers

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zhangpanyi/basebot/history"
	"github.com/zhangpanyi/basebot/logger"
	"github.com/zhangpanyi/basebot/telegram/methods"
	"github.com/zhangpanyi/basebot/telegram/types"
	"luckybot/app/config"
	"luckybot/app/fmath"
	"luckybot/app/logic/algo"
	"luckybot/app/monitor"
	"luckybot/app/storage/models"
)

// 匹配类型
var reMathType *regexp.Regexp

// 匹配金额
var reMathAmount *regexp.Regexp

// 匹配数量
var reMathNumber *regexp.Regexp

func init() {
	var err error
	reMathType, err = regexp.Compile("^/new/(rand|equal)/$")
	if err != nil {
		panic(err)
	}

	reMathAmount, err = regexp.Compile("^/new/(rand|equal)/([0-9]+\\.?[0-9]*)/$")
	if err != nil {
		panic(err)
	}

	reMathNumber, err = regexp.Compile("^/new/(rand|equal)/([0-9]+\\.?[0-9]*)/(\\d+)/$")
	if err != nil {
		panic(err)
	}
}

var (
	// 随机红包
	randLuckyMoney = "rand"
	// 普通红包
	equalLuckyMoney = "equal"
)

// 红包信息
type luckyMoneys struct {
	typ     string     // 红包类型
	amount  *big.Float // 红包金额
	number  int        // 红包个数
	message string     // 红包留言
}

// 红包类型转字符串
func luckyMoneysTypeToString(fromID int64, typ string) string {
	if typ == randLuckyMoney {
		return tr(fromID, "lng_new_rand")
	}
	return tr(fromID, "lng_new_equal")
}

// NewHandler 创建红包
type NewHandler struct {
}

// Handle 消息处理
func (handler *NewHandler) Handle(bot *methods.BotExt, r *history.History, update *types.Update) {
	// 回复选择红包类型
	data := update.CallbackQuery.Data
	if data == "/new/" {
		r.Clear()
		handler.replyChooseType(bot, update.CallbackQuery)
		return
	}

	// 回复输入红包金额
	info := luckyMoneys{}
	result := reMathType.FindStringSubmatch(data)
	if len(result) == 2 {
		info.typ = result[1]
		handler.replyEnterAmount(bot, r, &info, update)
		return
	}

	// 回复输入红包数量
	var ok bool
	result = reMathAmount.FindStringSubmatch(data)
	if len(result) == 3 {
		info.typ = result[1]
		info.amount, ok = big.NewFloat(0).SetString(result[2])
		if !ok {
			return
		}
		handler.replyEnterNumber(bot, r, &info, update, true)
		return
	}

	// 回复输入红包留言
	result = reMathNumber.FindStringSubmatch(data)
	if len(result) == 4 {
		info.typ = result[1]
		info.amount, ok = big.NewFloat(0).SetString(result[2])
		if !ok {
			return
		}
		number, _ := strconv.Atoi(result[3])
		info.number = number
		handler.replyEnterMessage(bot, r, &info, update)
		return
	}

	// 路由到其它处理模块
	newHandler := handler.route(bot, update.CallbackQuery)
	if newHandler == nil {
		return
	}
	newHandler.Handle(bot, r, update)
}

// 消息路由
func (handler *NewHandler) route(bot *methods.BotExt, query *types.CallbackQuery) Handler {
	return nil
}

// 返回上级
func backSuperior(data string) string {
	s := strings.Split(data, "/")
	if len(s) <= 2 {
		return "/main/"
	}
	return strings.Join(s[:len(s)-2], "/") + "/"
}

// 生成基本菜单
func makeBaseMenus(fromID int64, data string) *methods.InlineKeyboardMarkup {
	menus := [...]methods.InlineKeyboardButton{
		methods.InlineKeyboardButton{
			Text:         tr(fromID, "lng_new_cancel"),
			CallbackData: "/main/",
		},
		methods.InlineKeyboardButton{
			Text:         tr(fromID, "lng_back_superior"),
			CallbackData: backSuperior(data),
		},
	}
	return methods.MakeInlineKeyboardMarkupAuto(menus[:], 1)
}

// 回复输入选择类型
func (handler *NewHandler) replyChooseType(bot *methods.BotExt, query *types.CallbackQuery) {

	// 生成菜单列表
	data := query.Data
	fromID := query.From.ID
	menus := [...]methods.InlineKeyboardButton{
		methods.InlineKeyboardButton{
			Text:         tr(fromID, "lng_new_rand"),
			CallbackData: data + randLuckyMoney + "/",
		},
		methods.InlineKeyboardButton{
			Text:         tr(fromID, "lng_new_equal"),
			CallbackData: data + equalLuckyMoney + "/",
		},
		methods.InlineKeyboardButton{
			Text:         tr(fromID, "lng_back_superior"),
			CallbackData: "/main/",
		},
	}

	// 回复请求结果
	reply := tr(fromID, "lng_new_choose_type")
	markup := methods.MakeInlineKeyboardMarkup(menus[:], 2, 1)
	_ = bot.AnswerCallbackQuery(query, "", false, "", 0)
	_, _ = bot.EditMessageReplyMarkup(query.Message, reply, true, markup)
}

// 处理输入红包金额
func (handler *NewHandler) handleEnterAmount(bot *methods.BotExt, r *history.History,
	info *luckyMoneys, update *types.Update, enterAmount string) {

	// 生成菜单列表
	query := update.CallbackQuery
	fromID := query.From.ID

	// 处理错误
	data := query.Data
	handlerError := func(reply string) {
		r.Pop()
		_ = bot.AnswerCallbackQuery(query, "", false, "", 0)
		markup := makeBaseMenus(fromID, query.Data)
		_, _ = bot.SendMessage(fromID, reply, true, markup)
	}

	// 检查输入金额
	serveCfg := config.GetServe()
	amount, ok := big.NewFloat(0).SetString(enterAmount)
	if !ok || amount.Cmp(big.NewFloat(0)) <= 0 {
		handlerError(fmt.Sprintf(tr(fromID, "lng_new_set_amount_error"), serveCfg.Precision))
		return
	}

	// 检查小数点位数
	s := strings.Split(enterAmount, ".")
	if len(s) == 2 && len(s[1]) > serveCfg.Precision {
		handlerError(fmt.Sprintf(tr(fromID, "lng_new_set_amount_error"), serveCfg.Precision))
		return
	}

	// 检查帐户余额
	balance, _ := getUserBalance(fromID, serveCfg.Symbol)
	if amount.Cmp(balance) == 1 {
		reply := tr(fromID, "lng_new_set_amount_no_asset")
		handlerError(fmt.Sprintf(reply, serveCfg.Symbol, balance))
		return
	}

	// 更新下个操作状态
	r.Clear()
	info.amount = amount
	update.CallbackQuery.Data = data + enterAmount + "/"
	handler.replyEnterNumber(bot, r, info, update, false)
}

// 回复输入红包金额
func (handler *NewHandler) replyEnterAmount(bot *methods.BotExt, r *history.History, info *luckyMoneys,
	update *types.Update) {

	// 处理输入金额
	back, err := r.Back()
	if err == nil && back.Message != nil {
		handler.handleEnterAmount(bot, r, info, update, back.Message.Text)
		return
	}

	// 生成菜单列表
	query := update.CallbackQuery
	fromID := query.From.ID
	markup := makeBaseMenus(fromID, query.Data)

	// 回复请求结果
	r.Clear().Push(update)
	amountDesc := tr(fromID, "lng_new_total_amount")
	if info.typ == equalLuckyMoney {
		amountDesc = tr(fromID, "lng_new_unit_amount")
	}

	serveCfg := config.GetServe()
	answer := fmt.Sprintf(tr(fromID, "lng_new_set_amount_answer"), amountDesc, serveCfg.Precision)
	_ = bot.AnswerCallbackQuery(query, answer, false, "", 0)

	reply := tr(fromID, "lng_new_set_amount")
	amount, _ := getUserBalance(fromID, serveCfg.Symbol)
	reply = fmt.Sprintf(reply, amountDesc, serveCfg.Precision, luckyMoneysTypeToString(fromID, info.typ),
		serveCfg.Symbol, amount.String())
	_, _ = bot.EditMessageReplyMarkup(query.Message, reply, true, markup)
}

// 最低单个金额
func minSingleAmount() *big.Float {
	base := big.NewInt(10)
	serveCfg := config.GetServe()
	base.Exp(base, big.NewInt(int64(serveCfg.Precision)), nil)
	wei, _ := big.NewFloat(0).SetString(base.String())
	return wei.Quo(big.NewFloat(1), wei)
}

// 处理输入红包个数
func (handler *NewHandler) handleEnterNumber(bot *methods.BotExt, r *history.History,
	info *luckyMoneys, update *types.Update, enterNumber string) {

	// 生成菜单列表
	query := update.CallbackQuery
	fromID := query.From.ID

	// 处理错误
	handlerError := func(reply string) {
		r.Pop()
		markup := makeBaseMenus(fromID, query.Data)
		_ = bot.AnswerCallbackQuery(query, "", false, "", 0)
		_, _ = bot.SendMessage(fromID, reply, true, markup)
	}

	// 检查红包数量
	serveCfg := config.GetServe()
	number, err := strconv.Atoi(enterNumber)
	if err != nil || number <= 0 {
		handlerError(fmt.Sprintf(tr(fromID, "lng_new_set_number_error"), minSingleAmount().String()))
		return
	}

	// 检查账户余额
	balance, _ := getUserBalance(fromID, serveCfg.Symbol)
	if info.typ == equalLuckyMoney {
		if fmath.Mul(info.amount, big.NewFloat(float64(number))).Cmp(balance) == 1 {
			reply := tr(fromID, "lng_new_set_number_not_enough")
			handlerError(fmt.Sprintf(reply, serveCfg.Symbol, balance.String()))
			return
		}
	} else if info.typ == randLuckyMoney {
		base := big.NewInt(10)
		base.Exp(base, big.NewInt(int64(serveCfg.Precision)), nil)
		wei, _ := big.NewFloat(0).SetString(base.String())
		product := fmath.Mul(wei, info.amount)
		unit, _ := product.Int(big.NewInt(0))
		if unit.Cmp(big.NewInt(int64(number))) == -1 {
			reply := tr(fromID, "lng_new_set_number_not_enough")
			handlerError(fmt.Sprintf(reply, serveCfg.Symbol, balance.String()))
			return
		}
	}

	// 更新下个操作状态
	r.Clear()
	info.number = number
	update.CallbackQuery.Data += enterNumber + "/"
	handler.replyEnterMessage(bot, r, info, update)
}

// 回复输入红包数量
func (handler *NewHandler) replyEnterNumber(bot *methods.BotExt, r *history.History, info *luckyMoneys,
	update *types.Update, edit bool) {

	// 处理输入个数
	back, err := r.Back()
	if err == nil && back.Message != nil {
		handler.handleEnterNumber(bot, r, info, update, back.Message.Text)
		return
	}

	// 提示输入红包个数
	r.Clear().Push(update)
	query := update.CallbackQuery
	fromID := query.From.ID
	markup := makeBaseMenus(fromID, query.Data)

	amountDesc := tr(fromID, "lng_new_total_amount")
	if info.typ == equalLuckyMoney {
		amountDesc = tr(fromID, "lng_new_unit_amount")
	}

	serveCfg := config.GetServe()
	reply := tr(fromID, "lng_new_set_number")
	reply = fmt.Sprintf(reply, minSingleAmount().String(), luckyMoneysTypeToString(fromID, info.typ),
		amountDesc, info.amount.String(), serveCfg.Symbol)

	if !edit {
		_, _ = bot.SendMessage(fromID, reply, true, markup)
	} else {
		_, _ = bot.EditMessageReplyMarkup(query.Message, reply, true, markup)
	}
	_ = bot.AnswerCallbackQuery(query, tr(fromID, "lng_new_set_number_answer"), false, "", 0)
}

// 处理输入红包留言
func (handler *NewHandler) handleEnterMessage(bot *methods.BotExt, r *history.History,
	info *luckyMoneys, update *types.Update, message string) {

	// 处理错误
	query := update.CallbackQuery
	fromID := query.From.ID
	handlerError := func(reply string) {
		r.Pop()
		_ = bot.AnswerCallbackQuery(query, "", false, "", 0)
		markup := makeBaseMenus(fromID, query.Data)
		_, _ = bot.SendMessage(fromID, reply, true, markup)
		return
	}

	// 检查留言长度
	serve := config.GetServe()
	if len(message) == 0 || len(message) > serve.MaxMessageLen {
		reply := fmt.Sprintf(tr(fromID, "lng_new_set_message_error"),
			serve.MaxMessageLen)
		handlerError(reply)
		return
	}

	// 处理生成红包
	info.message = message
	data, err := handler.handleGenerateLuckyMoney(fromID, query.From.FirstName, info)
	if err != nil {
		logger.Warnf("Failed to create lucky money, %v", err)
		handlerError(tr(fromID, "lng_new_failed"))
		return
	}

	// 删除已有键盘
	remove := methods.ReplyKeyboardRemove{
		RemoveKeyboard: true,
	}
	_, _ = bot.SendMessage(fromID, tr(fromID, "lng_new_waiting"), false, &remove)

	// 回复红包内容
	r.Clear()
	reply := tr(fromID, "lng_new_created")
	reply = fmt.Sprintf(reply, bot.UserName)
	menus := [...]methods.InlineKeyboardButton{
		methods.InlineKeyboardButton{
			Text:              tr(fromID, "lng_send_luckymoney"),
			SwitchInlineQuery: data.SN,
		},
	}
	_ = bot.AnswerCallbackQuery(query, "", false, "", 0)
	markup := methods.MakeInlineKeyboardMarkupAuto(menus[:], 1)
	_, _ = bot.SendMessageDisableWebPagePreview(fromID, reply, true, markup)
}

// 回复输入红包留言
func (handler *NewHandler) replyEnterMessage(bot *methods.BotExt, r *history.History, info *luckyMoneys,
	update *types.Update) {

	// 处理输入留言
	back, err := r.Back()
	if err == nil && back.Message != nil {
		handler.handleEnterMessage(bot, r, info, update, back.Message.Text)
		return
	}

	// 生成回复键盘
	query := update.CallbackQuery
	fromID := query.From.ID
	menus := [...]methods.KeyboardButton{
		methods.KeyboardButton{
			Text: tr(fromID, "lng_new_benediction"),
		},
	}
	markup := methods.MakeReplyKeyboardMarkup(menus[:], 1)
	markup.OneTimeKeyboard = true

	// 提示输入红包留言
	r.Clear().Push(update)
	amount := tr(fromID, "lng_new_total_amount")
	if info.typ == equalLuckyMoney {
		amount = tr(fromID, "lng_new_unit_amount")
	}
	serveCfg := config.GetServe()
	reply := tr(fromID, "lng_new_set_message")
	reply = fmt.Sprintf(reply, luckyMoneysTypeToString(fromID, info.typ), serveCfg.Symbol,
		amount, info.amount.String(), serveCfg.Symbol, info.number)
	_, _ = bot.SendMessage(fromID, reply, true, markup)
	_ = bot.AnswerCallbackQuery(query, tr(fromID, "lng_new_set_message_answer"), false, "", 0)
}

// 处理生成红包
func (handler *NewHandler) handleGenerateLuckyMoney(userID int64, firstName string,
	info *luckyMoneys) (*models.LuckyMoney, error) {

	// 生成红包
	var luckyMoneyArr []*big.Float
	amount := big.NewFloat(0).Set(info.amount)
	if info.typ == equalLuckyMoney {
		amount.Mul(amount, big.NewFloat(float64(info.number)))
	}
	if info.typ == randLuckyMoney {
		var err error
		serveCfg := config.GetServe()
		luckyMoneyArr, err = algo.Generate(amount, serveCfg.Precision, info.number)
		if err != nil {
			logger.Errorf("Failed to generate lucky money, user_id: %v, %v", userID, err)
			return nil, err
		}
	} else {
		luckyMoneyArr = make([]*big.Float, 0, info.number)
		for i := 0; i < info.number; i++ {
			luckyMoneyArr = append(luckyMoneyArr, info.amount)
		}
	}

	// 锁定资金
	serveCfg := config.GetServe()
	model := models.AccountModel{}
	account, err := model.LockAccount(userID, serveCfg.Symbol, amount)
	if err != nil {
		return nil, err
	}
	logger.Errorf("Lock account, user_id: %v, asset: %v, amount: %v",
		userID, serveCfg.Symbol, amount.String())

	// 保存红包信息
	luckyMoney := models.LuckyMoney{
		SenderID:   userID,
		SenderName: firstName,
		Asset:      serveCfg.Symbol,
		Amount:     info.amount,
		Number:     uint32(info.number),
		Message:    info.message,
		Lucky:      info.typ == randLuckyMoney,
		Timestamp:  time.Now().UTC().Unix(),
	}
	if info.typ == equalLuckyMoney {
		luckyMoney.Value = big.NewFloat(0).Set(info.amount)
	}
	luckyMoneyModel := models.LuckyMoneyModel{}
	data, err := luckyMoneyModel.NewLuckyMoney(&luckyMoney, luckyMoneyArr)
	if err != nil {
		// 解锁资金
		if _, err := model.UnlockAccount(userID, serveCfg.Symbol, amount); err != nil {
			logger.Errorf("Failed to unlock asset, user_id: %v, asset: %v, amount: %v",
				userID, serveCfg.Symbol, amount.String())
		}
		logger.Errorf("Failed to new lucky money, user_id: %v, %v", userID, err)
		return nil, err
	}
	logger.Errorf("Generate lucky money, id: %v, user_id: %v, asset: %v, amount: %v",
		data.ID, userID, serveCfg.Symbol, amount.String())

	// 插入账户记录
	versionModel := models.AccountVersionModel{}
	_, _ = versionModel.InsertVersion(userID, &models.Version{
		Symbol:          serveCfg.Symbol,
		Locked:          amount,
		Amount:          account.Amount,
		Reason:          models.ReasonGive,
		RefLuckyMoneyID: &luckyMoney.ID,
	})

	// 添加到检查队列
	monitor.AddToQueue(luckyMoney.ID, luckyMoney.Timestamp)

	return data, nil
}
