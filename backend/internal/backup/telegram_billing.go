package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adp/panel/internal/billing"
	"github.com/adp/panel/internal/store"
)

// Reply-keyboard button labels for the billing menu. A tap arrives as a message
// whose text is exactly the label, so these must match the keyboard in
// clientKeyboardJSON.
const (
	statusButtonLabel  = "👤 Мой статус"
	topupButtonLabel   = "⭐ Пополнить"
	buyButtonLabel     = "🛒 Купить подписку"
	historyButtonLabel = "🧾 История"
	supportButtonLabel = "🆘 Поддержка"
)

// starTopupPresets are the Star amounts offered on the top-up screen.
var starTopupPresets = []int64{50, 100, 250, 500, 1000}

// handleBillingText handles the billing reply-keyboard buttons and slash commands
// for an already-billing-enabled bot. Returns true if it consumed the message.
func (t *Telegram) handleBillingText(ctx context.Context, chatID, username, text string, deliver func(context.Context, *store.Client)) bool {
	switch billingCommand(text) {
	case "status":
		t.sendStatus(ctx, chatID)
	case "topup":
		t.sendTopupMethods(ctx, chatID, username)
	case "buy":
		t.sendTariffs(ctx, chatID, username)
	case "history":
		t.sendHistory(ctx, chatID)
	case "support":
		t.sendSupport(ctx, chatID)
	default:
		return false
	}
	return true
}

// billingCommand maps a reply-keyboard label or a slash command to a canonical
// billing action, or "" if the text is not a billing command.
func billingCommand(text string) string {
	switch text {
	case statusButtonLabel:
		return "status"
	case topupButtonLabel:
		return "topup"
	case buyButtonLabel:
		return "buy"
	case historyButtonLabel:
		return "history"
	case supportButtonLabel:
		return "support"
	}
	cmd := strings.ToLower(strings.TrimSpace(text))
	if i := strings.IndexAny(cmd, " @"); i >= 0 {
		cmd = cmd[:i]
	}
	switch cmd {
	case "/status", "/balance":
		return "status"
	case "/topup", "/pay":
		return "topup"
	case "/buy", "/subscribe":
		return "buy"
	case "/history":
		return "history"
	case "/support":
		return "support"
	}
	return ""
}

// linkedClient resolves the client bound to chatID, messaging the user and
// returning false when the chat isn't linked yet.
func (t *Telegram) linkedClient(ctx context.Context, chatID string) (*store.Client, bool) {
	c, err := t.store.GetClientByTelegramChatID(ctx, chatID)
	if err != nil {
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Сначала привяжите аккаунт: откройте персональную ссылку, которую дал администратор. "+
				"(Link your account first — open the personal link your administrator gave you.)"))
		return nil, false
	}
	return c, true
}

// selfSignupOn reports whether strangers may buy a subscription on their own.
func (t *Telegram) selfSignupOn(ctx context.Context) bool {
	if t.billing == nil {
		return false
	}
	cfg, err := t.billing.GetSettings(ctx)
	return err == nil && cfg.SelfSignupEnabled
}

// payingClient resolves the client for a purchase, registering a new one when a
// stranger reaches this point and self-signup is on. Signup happens here rather
// than on /start so that merely opening the bot leaves no trace in the client
// list — only an actual move to pay does.
func (t *Telegram) payingClient(ctx context.Context, chatID, username string) (*store.Client, bool) {
	if c, err := t.store.GetClientByTelegramChatID(ctx, chatID); err == nil {
		return c, true
	}
	if !t.selfSignupOn(ctx) || t.signup == nil {
		return t.linkedClient(ctx, chatID)
	}
	c, err := t.signup.CreateSelfSignup(ctx, signupName(chatID, username), chatID, username)
	if err != nil {
		t.logger.Warn("billing: self-signup failed", "chat", chatID, "err", err)
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Не удалось создать аккаунт. Попробуйте позже. (Could not create your account — try again later.)"))
		return nil, false
	}
	t.logger.Info("billing: self-signup", "client", c.ID, "chat", chatID)
	_ = t.SendMessageTo(ctx, chatID, withFooter(
		"👤 Аккаунт создан. Доступ включится сразу после оплаты подписки.\n"+
			"(Account created — access switches on as soon as you buy a subscription.)"))
	return c, true
}

// signupName builds a readable client name from the Telegram handle, falling
// back to the chat id when the user has none. Names need not be unique.
func signupName(chatID, username string) string {
	if u := strings.TrimSpace(username); u != "" {
		return "tg:@" + u
	}
	return "tg:" + chatID
}

// sendWelcome greets a stranger with the plans on offer. Deliberately creates
// nothing: tapping a plan is what registers them (see payingClient).
func (t *Telegram) sendWelcome(ctx context.Context, chatID string) {
	tariffs, err := t.billing.Tariffs(ctx)
	if err != nil {
		return
	}
	rows := make([][]inlineButton, 0, len(tariffs)+1)
	for _, tf := range tariffs {
		rows = append(rows, []inlineButton{{
			Text:         fmt.Sprintf("%s — %s", tariffTitleRU(tf.Key), formatRubles(tf.PriceKopecks)),
			CallbackData: "buy:" + tf.Key,
		}})
	}
	rows = append(rows, []inlineButton{{Text: topupButtonLabel, CallbackData: "pay:stars"}})
	_ = t.sendMessageMarkup(ctx, chatID, withFooter(
		"<b>👋 Привет! Здесь можно купить доступ к VPN.</b>\n\n"+
			"Выберите тариф — аккаунт создастся автоматически, оплата проходит звёздами Telegram. "+
			"Сразу после оплаты сюда придёт ваш конфиг.\n"+
			"(Pick a plan — your account is created automatically and paid with Telegram Stars; "+
			"your config arrives here right after payment.)"),
		inlineKeyboardJSON(rows))
}

// sendStatus reports the client's wallet balance and subscription state.
func (t *Telegram) sendStatus(ctx context.Context, chatID string) {
	c, ok := t.linkedClient(ctx, chatID)
	if !ok {
		return
	}
	cb, err := t.billing.ClientBilling(ctx, c.ID, 0)
	if err != nil {
		_ = t.SendMessageTo(ctx, chatID, withFooter("Не удалось получить статус. (Could not load your status.)"))
		return
	}
	var b strings.Builder
	b.WriteString("<b>👤 Ваш статус (Your status)</b>\n\n")
	b.WriteString("Баланс (Balance): <b>" + formatRubles(cb.BalanceKopecks) + "</b>\n")
	switch {
	case cb.Active:
		b.WriteString("Подписка активна до (active until): <b>" + formatDateRU(cb.ActiveUntil) + "</b>\n")
	case cb.ActiveUntil != "":
		b.WriteString("Подписка истекла (expired): " + formatDateRU(cb.ActiveUntil) + "\n" +
			"Доступ приостановлен — купите подписку, чтобы возобновить. (Access suspended — buy a subscription to resume.)\n")
	default:
		b.WriteString("Активной подписки нет (no active subscription). Нажмите «" + buyButtonLabel + "». (Tap the button to buy one.)\n")
	}
	_ = t.SendMessageTo(ctx, chatID, withFooter(b.String()))
}

// sendTopupMethods shows the payment-method chooser (Stars now; the rest soon).
func (t *Telegram) sendTopupMethods(ctx context.Context, chatID, username string) {
	if _, ok := t.payingClient(ctx, chatID, username); !ok {
		return
	}
	kb := inlineKeyboardJSON([][]inlineButton{
		{{Text: "⭐ Telegram Stars", CallbackData: "pay:stars"}},
		{{Text: "💳 Карта — скоро (soon)", CallbackData: "pay:soon:card"}},
		{{Text: "🏦 СБП — скоро (soon)", CallbackData: "pay:soon:sbp"}},
		{{Text: "🪙 Крипта — скоро (soon)", CallbackData: "pay:soon:crypto"}},
	})
	_ = t.sendMessageMarkup(ctx, chatID, withFooter(
		"<b>⭐ Пополнение баланса (Top up)</b>\n\n"+
			"Выберите способ оплаты. Сейчас доступны Telegram Stars; карта, СБП и крипта — скоро.\n"+
			"(Choose a payment method. Telegram Stars is available now; card, SBP and crypto are coming soon.)"), kb)
}

// sendStarAmounts shows the Star top-up buttons: first one per tariff, sized to
// exactly cover what this client still needs for that plan, then round presets.
//
// The tariff-sized buttons exist because raw presets and ruble prices don't line
// up — at the default rate no preset reaches the yearly tariff at all, so a
// client picking from presets alone would have to pay twice and overpay.
func (t *Telegram) sendStarAmounts(ctx context.Context, chatID string, c *store.Client) {
	cfg, err := t.billing.GetSettings(ctx)
	if err != nil {
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Не удалось загрузить тарифы. Попробуйте позже. (Could not load the plans — try again later.)"))
		return
	}
	cb, err := t.billing.ClientBilling(ctx, c.ID, 0)
	if err != nil {
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Не удалось загрузить баланс. Попробуйте позже. (Could not load your balance — try again later.)"))
		return
	}
	tariffs, err := t.billing.Tariffs(ctx)
	if err != nil {
		tariffs = nil
	}

	rows := make([][]inlineButton, 0, len(tariffs)+len(starTopupPresets))
	covered := 0
	for _, tf := range tariffs {
		need := tf.PriceKopecks - cb.BalanceKopecks
		if need <= 0 {
			covered++ // balance already buys this plan — no top-up needed
			continue
		}
		stars := starsFor(need, cfg.StarRateKopecks)
		if stars <= 0 {
			continue
		}
		rows = append(rows, []inlineButton{{
			Text:         fmt.Sprintf("🛒 %s — %d ⭐ ≈ %s", tariffTitleRU(tf.Key), stars, formatRubles(stars*cfg.StarRateKopecks)),
			CallbackData: fmt.Sprintf("pay:stars:%d", stars),
		}})
	}
	for _, n := range starTopupPresets {
		rows = append(rows, []inlineButton{{
			Text:         fmt.Sprintf("%d ⭐ ≈ %s", n, formatRubles(n*cfg.StarRateKopecks)),
			CallbackData: fmt.Sprintf("pay:stars:%d", n),
		}})
	}

	head := "<b>⭐ Оплата звёздами Telegram (Pay with Telegram Stars)</b>\n\n" +
		"Ваш баланс (Your balance): <b>" + formatRubles(cb.BalanceKopecks) + "</b>\n"
	switch {
	case covered == len(tariffs) && len(tariffs) > 0:
		head += "Баланса уже хватает на любой тариф — можно сразу нажать «" + buyButtonLabel + "».\n" +
			"(Your balance already covers every plan — just buy one.)"
	default:
		head += "Верхние кнопки пополняют ровно на выбранный тариф с учётом баланса; ниже — произвольные суммы.\n" +
			"(The top buttons top up exactly enough for that plan, given your balance; round amounts below.)"
	}
	_ = t.sendMessageMarkup(ctx, chatID, withFooter(head), inlineKeyboardJSON(rows))
}

// starsFor returns the smallest whole number of Stars whose ruble value covers
// needKopecks at rateKopecks per Star (rounding up — a short top-up would leave
// the client unable to buy the plan they picked the button for).
func starsFor(needKopecks, rateKopecks int64) int64 {
	if needKopecks <= 0 || rateKopecks <= 0 {
		return 0
	}
	return (needKopecks + rateKopecks - 1) / rateKopecks
}

// sendStarInvoice sends a Telegram Stars invoice for a top-up of stars Stars.
func (t *Telegram) sendStarInvoice(ctx context.Context, chatID string, c *store.Client, stars int64) {
	cfg, _ := t.billing.GetSettings(ctx)
	desc := fmt.Sprintf("Пополнение на %d ⭐ (≈ %s) для %s", stars, formatRubles(stars*cfg.StarRateKopecks), c.Name)
	payload := fmt.Sprintf("topup:%d:%d", c.ID, stars)
	if err := t.sendInvoiceStars(ctx, chatID, "Пополнение баланса", desc, payload, stars); err != nil {
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Не удалось создать счёт. Попробуйте позже. (Could not create the invoice — try again later.)"))
	}
}

// sendTariffs shows the subscription plans as buttons plus the current balance.
func (t *Telegram) sendTariffs(ctx context.Context, chatID, username string) {
	c, ok := t.payingClient(ctx, chatID, username)
	if !ok {
		return
	}
	tariffs, err := t.billing.Tariffs(ctx)
	if err != nil {
		return
	}
	cb, _ := t.billing.ClientBilling(ctx, c.ID, 0)
	rows := make([][]inlineButton, 0, len(tariffs))
	for _, tf := range tariffs {
		rows = append(rows, []inlineButton{{
			Text:         fmt.Sprintf("%s — %s", tariffTitleRU(tf.Key), formatRubles(tf.PriceKopecks)),
			CallbackData: "buy:" + tf.Key,
		}})
	}
	_ = t.sendMessageMarkup(ctx, chatID, withFooter(
		"<b>🛒 Купить подписку (Buy subscription)</b>\n\n"+
			"Ваш баланс (Your balance): <b>"+formatRubles(cb.BalanceKopecks)+"</b>\n"+
			"Выберите тариф — сумма спишется с баланса. (Choose a plan — it is charged to your balance.)"),
		inlineKeyboardJSON(rows))
}

// sendHistory lists the client's recent wallet operations.
func (t *Telegram) sendHistory(ctx context.Context, chatID string) {
	c, ok := t.linkedClient(ctx, chatID)
	if !ok {
		return
	}
	cb, err := t.billing.ClientBilling(ctx, c.ID, 10)
	if err != nil {
		return
	}
	var b strings.Builder
	b.WriteString("<b>🧾 История операций (History)</b>\n\n")
	if len(cb.Transactions) == 0 {
		b.WriteString("Пока пусто. (Nothing yet.)")
	} else {
		for _, tx := range cb.Transactions {
			b.WriteString(formatDateRU(tx.CreatedAt) + " — " + txLabel(tx) + "\n")
		}
	}
	_ = t.SendMessageTo(ctx, chatID, withFooter(b.String()))
}

// sendSupport shows the operator's support contact, with a link button for @handles.
func (t *Telegram) sendSupport(ctx context.Context, chatID string) {
	cfg, _ := t.billing.GetSettings(ctx)
	contact := strings.TrimSpace(cfg.SupportContact)
	if contact == "" {
		contact = "@solepytt"
	}
	msg := "<b>🆘 Поддержка (Support)</b>\n\nНапишите нам: " + html.EscapeString(contact) +
		"\n(Contact us at the handle above.)"
	var kb string
	if strings.HasPrefix(contact, "@") && len(contact) > 1 {
		kb = inlineKeyboardJSON([][]inlineButton{{{
			Text: "Написать в поддержку (Contact)", URL: "https://t.me/" + contact[1:],
		}}})
	}
	_ = t.sendMessageMarkup(ctx, chatID, withFooter(msg), kb)
}

// handleCallback processes an inline-button tap (payment method, Star amount,
// tariff purchase). It always answers the callback to clear the client's spinner.
func (t *Telegram) handleCallback(ctx context.Context, cq *tgCallbackQuery, deliver func(context.Context, *store.Client)) {
	if cq.Message == nil {
		t.answerCallback(ctx, cq.ID, "", false)
		return
	}
	chatID := strconv.FormatInt(cq.Message.Chat.ID, 10)
	username := cq.From.Username
	if !t.billingOn(ctx) {
		t.answerCallback(ctx, cq.ID, "Оплата сейчас недоступна. (Payments are unavailable.)", true)
		return
	}
	data := cq.Data
	switch {
	case data == "pay:stars":
		t.answerCallback(ctx, cq.ID, "", false)
		c, ok := t.payingClient(ctx, chatID, username)
		if !ok {
			return
		}
		t.sendStarAmounts(ctx, chatID, c)
	case strings.HasPrefix(data, "pay:stars:"):
		t.answerCallback(ctx, cq.ID, "", false)
		c, ok := t.payingClient(ctx, chatID, username)
		if !ok {
			return
		}
		n, err := strconv.ParseInt(strings.TrimPrefix(data, "pay:stars:"), 10, 64)
		if err != nil || n <= 0 {
			return
		}
		t.sendStarInvoice(ctx, chatID, c, n)
	case strings.HasPrefix(data, "pay:soon:"):
		t.answerCallback(ctx, cq.ID, "Скоро — пока доступны Telegram Stars. (Coming soon — use Telegram Stars for now.)", true)
	case strings.HasPrefix(data, "buy:"):
		t.answerCallback(ctx, cq.ID, "", false)
		t.handleBuy(ctx, chatID, username, strings.TrimPrefix(data, "buy:"), deliver)
	default:
		t.answerCallback(ctx, cq.ID, "", false)
	}
}

// handleBuy charges a tariff to the client's wallet and confirms, or explains a
// shortfall and offers a top-up.
func (t *Telegram) handleBuy(ctx context.Context, chatID, username, key string, deliver func(context.Context, *store.Client)) {
	c, ok := t.payingClient(ctx, chatID, username)
	if !ok {
		return
	}
	tariff, exists := t.billing.TariffByKey(ctx, key)
	if !exists {
		_ = t.SendMessageTo(ctx, chatID, withFooter("Неизвестный тариф. (Unknown plan.)"))
		return
	}
	cb, err := t.billing.Purchase(ctx, c.ID, key)
	if errors.Is(err, store.ErrInsufficientFunds) {
		before, _ := t.billing.ClientBilling(ctx, c.ID, 0)
		_ = t.SendMessageTo(ctx, chatID, withFooter(fmt.Sprintf(
			"Недостаточно средств: тариф «%s» стоит %s, на балансе %s. Пополните баланс.\n"+
				"(Insufficient funds — top up your balance.)",
			tariffTitleRU(key), formatRubles(tariff.PriceKopecks), formatRubles(before.BalanceKopecks))))
		t.sendTopupMethods(ctx, chatID, "")
		return
	}
	if err != nil {
		t.logger.Warn("billing: purchase failed", "client", c.ID, "err", err)
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Не удалось оформить подписку. Попробуйте позже. (Could not complete the purchase — try again later.)"))
		return
	}
	_ = t.SendMessageTo(ctx, chatID, withFooter(fmt.Sprintf(
		"✅ Подписка «%s» активна до <b>%s</b>. Доступ включён. Остаток: %s.\n"+
			"(Subscription active until the date above; access enabled.)",
		tariffTitleRU(key), formatDateRU(cb.ActiveUntil), formatRubles(cb.BalanceKopecks))))
	// Re-push the (now enabled) config so the client has fresh links.
	if deliver != nil {
		deliver(ctx, c)
	}
}

// handlePreCheckout approves a Stars payment after basic validation. Telegram
// gives us 10 seconds to answer or the charge is auto-cancelled.
func (t *Telegram) handlePreCheckout(ctx context.Context, pcq *tgPreCheckout) {
	if !t.billingOn(ctx) {
		_ = t.answerPreCheckout(ctx, pcq.ID, false, "Оплата сейчас недоступна. (Payments are currently unavailable.)")
		return
	}
	clientID, _, ok := parseTopupPayload(pcq.InvoicePayload)
	if !ok || pcq.Currency != "XTR" {
		_ = t.answerPreCheckout(ctx, pcq.ID, false, "Некорректный платёж. (Invalid payment.)")
		return
	}
	if _, err := t.store.GetClient(ctx, clientID); err != nil {
		_ = t.answerPreCheckout(ctx, pcq.ID, false, "Клиент не найден. (Client not found.)")
		return
	}
	_ = t.answerPreCheckout(ctx, pcq.ID, true, "")
}

// handleSuccessfulPayment credits a confirmed Stars top-up to the client's wallet
// (idempotently by charge id) and confirms.
func (t *Telegram) handleSuccessfulPayment(ctx context.Context, msg *tgMessage, deliver func(context.Context, *store.Client)) {
	if t.billing == nil {
		return
	}
	sp := msg.SuccessfulPayment
	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	clientID, _, ok := parseTopupPayload(sp.InvoicePayload)
	if !ok {
		return
	}
	stars := sp.TotalAmount
	credited, newBal, err := t.billing.CreditStars(ctx, clientID, stars, sp.TelegramPaymentChargeID)
	if errors.Is(err, store.ErrDuplicateCharge) {
		return // already credited — nothing to do
	}
	if err != nil {
		t.logger.Warn("billing: credit stars failed", "client", clientID, "err", err)
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Платёж получен, но зачислить баланс не удалось. Свяжитесь с поддержкой. "+
				"(Payment received but crediting failed — please contact support.)"))
		return
	}
	_ = t.SendMessageTo(ctx, chatID, withFooter(fmt.Sprintf(
		"✅ Баланс пополнен на %s (%d ⭐). Текущий баланс: <b>%s</b>.\n(Balance topped up.)",
		formatRubles(credited), stars, formatRubles(newBal))))
	// Offer to spend the fresh balance on a subscription.
	t.sendTariffs(ctx, chatID, "")
}

// parseTopupPayload parses an invoice payload of the form "topup:<clientID>:<stars>".
func parseTopupPayload(payload string) (clientID, stars int64, ok bool) {
	parts := strings.Split(payload, ":")
	if len(parts) != 3 || parts[0] != "topup" {
		return 0, 0, false
	}
	cid, err1 := strconv.ParseInt(parts[1], 10, 64)
	st, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil || cid <= 0 || st <= 0 {
		return 0, 0, false
	}
	return cid, st, true
}

// --- Bot API calls specific to billing ---

// inlineButton is one inline-keyboard button (callback or URL).
type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

func inlineKeyboardJSON(rows [][]inlineButton) string {
	b, _ := json.Marshal(map[string]any{"inline_keyboard": rows})
	return string(b)
}

// sendMessageMarkup sends an HTML message with an optional reply markup (a JSON
// object string, e.g. an inline keyboard).
func (t *Telegram) sendMessageMarkup(ctx context.Context, chatID, htmlText, replyMarkup string) error {
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"chat_id": chatID, "text": htmlText, "parse_mode": "HTML", "disable_web_page_preview": true,
	}
	if replyMarkup != "" {
		payload["reply_markup"] = json.RawMessage(replyMarkup)
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/sendMessage", t.apiBase, token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return tgDo(t.client, req)
}

// sendInvoiceStars sends a Telegram Stars invoice (currency XTR, empty provider
// token, exactly one price whose amount is the plain Star count).
func (t *Telegram) sendInvoiceStars(ctx context.Context, chatID, title, description, payload string, stars int64) error {
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"chat_id":        chatID,
		"title":          title,
		"description":    description,
		"payload":        payload,
		"provider_token": "",
		"currency":       "XTR",
		"prices":         []map[string]any{{"label": fmt.Sprintf("%d ⭐", stars), "amount": stars}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/sendInvoice", t.apiBase, token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return tgDo(t.client, req)
}

// RefundStarPayment returns a Stars payment to the payer. userID is the payer's
// Telegram id — for the private chats this bot works in, that is the chat id we
// store on the client. Satisfies billing.StarRefunder.
func (t *Telegram) RefundStarPayment(ctx context.Context, userID, chargeID string) error {
	uid, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: bad user id %q: %w", userID, err)
	}
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"user_id":                    uid,
		"telegram_payment_charge_id": chargeID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/refundStarPayment", t.apiBase, token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return tgDo(t.client, req)
}

// answerPreCheckout responds to a pre_checkout_query (must happen within 10s).
func (t *Telegram) answerPreCheckout(ctx context.Context, queryID string, ok bool, errMsg string) error {
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{"pre_checkout_query_id": queryID, "ok": ok}
	if !ok {
		payload["error_message"] = errMsg
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/answerPreCheckoutQuery", t.apiBase, token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return tgDo(t.client, req)
}

// answerCallback acknowledges an inline-button tap (best-effort), optionally
// showing a toast/alert.
func (t *Telegram) answerCallback(ctx context.Context, callbackID, text string, alert bool) {
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return
	}
	payload := map[string]any{"callback_query_id": callbackID}
	if text != "" {
		payload["text"] = text
	}
	if alert {
		payload["show_alert"] = true
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/answerCallbackQuery", t.apiBase, token), bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	_ = tgDo(t.client, req)
}

// --- formatting helpers ---

// formatRubles renders kopecks as a ruble string, e.g. 20000 -> "200 ₽",
// 7050 -> "70,50 ₽", -20000 -> "−200 ₽".
func formatRubles(kopecks int64) string {
	neg := kopecks < 0
	if neg {
		kopecks = -kopecks
	}
	rub, kop := kopecks/100, kopecks%100
	var s string
	if kop == 0 {
		s = fmt.Sprintf("%d ₽", rub)
	} else {
		s = fmt.Sprintf("%d,%02d ₽", rub, kop)
	}
	if neg {
		return "−" + s
	}
	return s
}

// formatRublesSigned always shows an explicit + or − sign.
func formatRublesSigned(kopecks int64) string {
	if kopecks >= 0 {
		return "+" + formatRubles(kopecks)
	}
	return formatRubles(kopecks)
}

func formatDateRU(iso string) string {
	if iso == "" {
		return "—"
	}
	tm, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return tm.UTC().Format("02.01.2006 15:04") + " UTC"
}

func tariffTitleRU(key string) string {
	switch key {
	case "week":
		return "Неделя"
	case "month":
		return "Месяц"
	case "year":
		return "Год"
	}
	return key
}

// txLabel renders one ledger entry for the in-bot history.
func txLabel(tx billing.Transaction) string {
	amt := formatRublesSigned(tx.AmountKopecks)
	switch tx.Kind {
	case "topup":
		if tx.Stars > 0 {
			return fmt.Sprintf("Пополнение %s (%d ⭐)", amt, tx.Stars)
		}
		return "Пополнение " + amt
	case "purchase":
		return "Покупка подписки " + amt
	case "grant":
		return "Бонусная подписка (grant)"
	case "adjust":
		return "Коррекция баланса " + amt
	case "refund":
		return "Возврат " + amt
	}
	return amt
}
