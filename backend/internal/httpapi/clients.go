package httpapi

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"

	skipqr "github.com/skip2/go-qrcode"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/backup"
	"github.com/adp/panel/internal/brand"
	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/mail"
	"github.com/adp/panel/internal/store"
	syncpkg "github.com/adp/panel/internal/sync"
)

type clientsHandler struct {
	svc         *clients.Service
	store       *store.Store
	subBase     string
	sync        *syncpkg.Service
	mailer      *mail.Mailer
	telegram    *backup.Telegram
	emailBackup *backup.Email
}

// brandImageURL is the public URL of the mascot PNG the panel serves, used in
// HTML e-mail footers. Empty if no public base URL is configured.
func (h *clientsHandler) brandImageURL() string {
	if h.subBase == "" {
		return ""
	}
	return strings.TrimRight(h.subBase, "/") + "/cheremsha.png"
}

// emailSubject / emailBody / emailHTMLBody render the "here is your VPN
// subscription" message sent to a client's contact address, bilingually
// (Russian, with an English translation in parentheses).
func emailSubject(name string) string {
	s := "Ваша VPN-подписка (Your VPN subscription)"
	if name != "" {
		s += " — " + name
	}
	return s
}

func (h *clientsHandler) emailBody(c store.Client) string {
	url := h.subURL(c.SubscriptionToken)
	var b strings.Builder
	if c.Name != "" {
		b.WriteString("Здравствуйте, " + c.Name + "! (Hello, " + c.Name + "!)\n\n")
	}
	b.WriteString("Ваша персональная ссылка-подписка. Добавьте её в VPN-приложение ")
	b.WriteString("(sing-box, Hiddify, v2rayN, Streisand и т.п.), чтобы автоматически получать все конфигурации:\n")
	b.WriteString("(Your personal subscription link — add it to your VPN app to receive all configs automatically.)\n\n")
	b.WriteString(url + "\n\n")
	b.WriteString("Храните ссылку в тайне — любой, у кого она есть, получит ваш доступ. ")
	b.WriteString("Если она утекла, попросите оператора перевыпустить токен.\n")
	b.WriteString("(Keep this link private — anyone who has it can use your access. If it leaks, ask the operator to rotate your token.)\n\n")
	b.WriteString(brand.TextFooter())
	return b.String()
}

func (h *clientsHandler) emailHTMLBody(c store.Client) string {
	url := h.subURL(c.SubscriptionToken)
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<div style="font-family:sans-serif;font-size:15px;color:#222;line-height:1.5">`)
	if c.Name != "" {
		b.WriteString("<p>Здравствуйте, <b>" + esc(c.Name) + `</b>! <span style="color:#888">(Hello, ` + esc(c.Name) + "!)</span></p>")
	}
	b.WriteString(`<p>Ваша персональная ссылка-подписка. Добавьте её в VPN-приложение ` +
		`(sing-box, Hiddify, v2rayN, Streisand и т.п.), чтобы автоматически получать все конфигурации:` +
		`<br><span style="color:#888">(Your personal subscription link — add it to your VPN app to receive all configs automatically.)</span></p>`)
	b.WriteString(`<p style="margin:16px 0"><a href="` + esc(url) + `" style="word-break:break-all">` + esc(url) + `</a></p>`)
	b.WriteString(`<p style="color:#666;font-size:13px">Храните ссылку в тайне — любой, у кого она есть, получит ваш доступ. ` +
		`<span style="color:#999">(Keep this link private; if it leaks, ask the operator to rotate your token.)</span></p>`)
	b.WriteString(brand.HTMLFooter(h.brandImageURL()))
	b.WriteString(`</div>`)
	return b.String()
}

// sendEmail e-mails the client their subscription link on demand.
func (h *clientsHandler) sendEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client")
		return
	}
	if c.Email == "" {
		writeError(w, http.StatusBadRequest, "this client has no e-mail address")
		return
	}
	if h.mailer == nil || !h.mailer.Configured(r.Context()) {
		writeError(w, http.StatusBadRequest, "SMTP is not configured; set it up in Settings")
		return
	}
	if err := h.mailer.SendMessage(r.Context(), mail.Message{
		To: []string{c.Email}, Subject: emailSubject(c.Name),
		Text: h.emailBody(*c), HTML: h.emailHTMLBody(*c),
	}); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	h.audit(r, "client.email", c.ID, "{}")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// telegramLink returns the client's personal Telegram deep link. Opening it and
// pressing Start binds the user's Telegram chat to this client (handled by the
// bot's link poller); after that the panel can push their config to Telegram.
func (h *clientsHandler) telegramLink(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.svc.Get(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client")
		return
	}
	if h.telegram == nil {
		writeError(w, http.StatusBadRequest, "Telegram bot is not configured (Settings → Backup)")
		return
	}
	username, err := h.telegram.BotUsername(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, "set up the Telegram bot first (Settings → Backup)")
		return
	}
	token, err := h.svc.EnsureTelegramLinkToken(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build link")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"link":         fmt.Sprintf("https://t.me/%s?start=%s", username, token),
		"bot_username": username,
	})
}

// telegramConfig pushes the client's subscription link + QR to their linked chat.
func (h *clientsHandler) telegramConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client")
		return
	}
	if c.TelegramChatID == "" {
		writeError(w, http.StatusBadRequest, "this client hasn't linked their Telegram yet")
		return
	}
	if h.telegram == nil {
		writeError(w, http.StatusBadRequest, "Telegram bot is not configured (Settings → Backup)")
		return
	}
	url, qr, err := h.svc.SubscriptionConfig(r.Context(), id, h.subBase)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.telegram.SendClientConfig(r.Context(), c.TelegramChatID, c.Name, url, qr); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	h.audit(r, "client.telegram_config", c.ID, "{}")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// telegramUnlink forgets the client's linked Telegram chat.
func (h *clientsHandler) telegramUnlink(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.UnlinkTelegram(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink")
		return
	}
	h.audit(r, "client.telegram_unlink", c.ID, "{}")
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

// qrFileName makes a safe PNG filename from a client name.
func qrFileName(name string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
	if safe == "" {
		safe = "client"
	}
	return safe + ".png"
}

// sendConfigs bulk-sends every client's subscription link + QR code to the
// operator's backup channels (the e-mail and Telegram set up for backups): one
// e-mail with all links and the QR PNGs attached, plus one Telegram photo per
// client whose caption carries the link in <code> (tap-to-copy in Telegram).
func (h *clientsHandler) sendConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	emailOK := h.mailer != nil && h.mailer.Configured(ctx)
	tgOK := h.telegram != nil && h.telegram.CanSend(ctx)
	if !emailOK && !tgOK {
		writeError(w, http.StatusBadRequest, "set up Telegram or e-mail first (Settings → Backup)")
		return
	}

	clientsList, err := h.svc.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clients")
		return
	}
	type cfg struct {
		name, url string
		qr        []byte
	}
	items := make([]cfg, 0, len(clientsList))
	for _, c := range clientsList {
		u := h.subURL(c.SubscriptionToken)
		if u == "" {
			continue
		}
		png, err := skipqr.Encode(u, skipqr.Medium, 512)
		if err != nil {
			continue
		}
		items = append(items, cfg{name: c.Name, url: u, qr: png})
	}
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "no clients with a subscription link")
		return
	}

	resp := map[string]any{"total": len(items), "email_configured": emailOK, "telegram_configured": tgOK}

	if emailOK {
		to := ""
		if h.emailBackup != nil {
			if st, e := h.emailBackup.Status(ctx); e == nil {
				to = st.To
			}
		}
		if to == "" {
			resp["email_error"] = "no backup e-mail recipient set"
		} else {
			var text strings.Builder
			var htmlB strings.Builder
			text.WriteString("Ссылки-подписки всех клиентов (QR-коды во вложениях).\n")
			text.WriteString("(Subscription links for all clients; QR codes attached as PNGs.)\n\n")
			htmlB.WriteString(`<div style="font-family:sans-serif;font-size:14px;color:#222;line-height:1.5">`)
			htmlB.WriteString(`<p>Ссылки-подписки всех клиентов (QR-коды во вложениях).<br>` +
				`<span style="color:#888">(Subscription links for all clients; QR codes attached as PNGs.)</span></p><ul>`)
			atts := make([]mail.Attachment, 0, len(items))
			for _, it := range items {
				text.WriteString(it.name + "\n" + it.url + "\n\n")
				htmlB.WriteString(`<li><b>` + html.EscapeString(it.name) + `</b><br><a href="` +
					html.EscapeString(it.url) + `" style="word-break:break-all">` + html.EscapeString(it.url) + `</a></li>`)
				atts = append(atts, mail.Attachment{Filename: qrFileName(it.name), Data: it.qr, ContentType: "image/png"})
			}
			text.WriteString(brand.TextFooter())
			htmlB.WriteString(`</ul>` + brand.HTMLFooter(h.brandImageURL()) + `</div>`)
			if err := h.mailer.SendMessage(ctx, mail.Message{
				To: []string{to}, Subject: "Подписки клиентов (Client subscriptions)",
				Text: text.String(), HTML: htmlB.String(), Attachments: atts,
			}); err != nil {
				resp["email_error"] = err.Error()
			} else {
				resp["email_sent"] = true
				resp["email_to"] = to
			}
		}
	}

	if tgOK {
		sent, failed := 0, 0
		for _, it := range items {
			caption := fmt.Sprintf("<b>%s</b>\n<code>%s</code>", html.EscapeString(it.name), html.EscapeString(it.url))
			if err := h.telegram.SendPhoto(ctx, qrFileName(it.name), it.qr, caption); err != nil {
				failed++
			} else {
				sent++
			}
		}
		resp["telegram_sent"] = sent
		if failed > 0 {
			resp["telegram_failed"] = failed
		}
	}

	adminID, _ := auth.AdminIDFrom(ctx)
	recordAudit(ctx, h.store, r, adminID, "client.send_configs", "client", 0, "")
	writeJSON(w, http.StatusOK, resp)
}

// autoSync re-pushes config to all installed nodes after a client-level change
// (grants, enable/disable, delete). Sync is idempotent, so unaffected nodes no-op.
func (h *clientsHandler) autoSync() {
	if h.sync != nil {
		h.sync.AsyncAll()
	}
}

// --- DTOs (mirror docs/openapi.yaml) ---

type clientDTO struct {
	ID                int64            `json:"id"`
	Name              string           `json:"name"`
	UUID              string           `json:"uuid"`
	Password          string           `json:"password,omitempty"`
	SubscriptionToken string           `json:"subscription_token"`
	SubscriptionURL   string           `json:"subscription_url,omitempty"`
	Enabled           bool             `json:"enabled"`
	Remark            string           `json:"remark,omitempty"`
	Email             string           `json:"email,omitempty"`
	TelegramLinked    bool             `json:"telegram_linked"`
	TelegramUsername  string           `json:"telegram_username,omitempty"`
	InboundIDs        []int64          `json:"inbound_ids"`
	Grants            []clientGrantDTO `json:"grants,omitempty"`
	CreatedAt         string           `json:"created_at,omitempty"`
	UpdatedAt         string           `json:"updated_at,omitempty"`
}

type clientGrantDTO struct {
	InboundID  int64  `json:"inbound_id"`
	ServerID   int64  `json:"server_id"`
	ServerName string `json:"server_name,omitempty"`
	InboundTag string `json:"inbound_tag,omitempty"`
	Protocol   string `json:"protocol"`
	Enabled    bool   `json:"enabled"`
}

type clientLinkDTO struct {
	InboundID  int64  `json:"inbound_id"`
	ServerName string `json:"server_name,omitempty"`
	Protocol   string `json:"protocol"`
	Remark     string `json:"remark,omitempty"`
	URI        string `json:"uri"`
}

type clientInput struct {
	Name   string `json:"name"`
	Remark string `json:"remark"`
	Email  string `json:"email"`
}

type clientInboundsInput struct {
	InboundIDs []int64 `json:"inbound_ids"`
}

func (h *clientsHandler) subURL(token string) string {
	if h.subBase == "" {
		return ""
	}
	return strings.TrimRight(h.subBase, "/") + "/sub/" + token
}

// toDTO maps a client, loading its grant ids (and optionally full grants).
func (h *clientsHandler) toDTO(ctx context.Context, c *store.Client, withGrants bool) clientDTO {
	ids, _ := h.svc.GrantedInboundIDs(ctx, c.ID)
	if ids == nil {
		ids = []int64{}
	}
	dto := clientDTO{
		ID: c.ID, Name: c.Name, UUID: c.UUID, Password: c.Password,
		SubscriptionToken: c.SubscriptionToken, SubscriptionURL: h.subURL(c.SubscriptionToken),
		Enabled: c.Enabled, Remark: c.Remark, Email: c.Email, InboundIDs: ids,
		TelegramLinked: c.TelegramChatID != "", TelegramUsername: c.TelegramUsername,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if withGrants {
		grants, _ := h.svc.Grants(ctx, c.ID)
		dto.Grants = make([]clientGrantDTO, 0, len(grants))
		for _, g := range grants {
			dto.Grants = append(dto.Grants, clientGrantDTO{
				InboundID: g.InboundID, ServerID: g.ServerID, ServerName: g.ServerName,
				InboundTag: g.InboundTag, Protocol: g.Protocol, Enabled: g.GrantEnabled && g.InboundOn,
			})
		}
	}
	return dto
}

func (h *clientsHandler) audit(r *http.Request, action string, targetID int64, detail string) {
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, action, "client", targetID, detail)
}

// --- Handlers ---

func (h *clientsHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clients")
		return
	}
	out := make([]clientDTO, 0, len(list))
	for i := range list {
		out = append(out, h.toDTO(r.Context(), &list[i], false))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *clientsHandler) create(w http.ResponseWriter, r *http.Request) {
	var in clientInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	c, err := h.svc.Create(r.Context(), clients.Input{Name: in.Name, Remark: in.Remark, Email: strings.TrimSpace(in.Email)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create client")
		return
	}
	h.audit(r, "client.create", c.ID, `{"name":`+strconv.Quote(c.Name)+`}`)
	// If a contact e-mail was provided and SMTP is set up, send the subscription.
	if c.Email != "" && h.mailer != nil && h.mailer.Configured(r.Context()) {
		go func(cl store.Client) {
			_ = h.mailer.SendMessage(context.Background(), mail.Message{
				To: []string{cl.Email}, Subject: emailSubject(cl.Name),
				Text: h.emailBody(cl), HTML: h.emailHTMLBody(cl),
			})
		}(*c)
	}
	writeJSON(w, http.StatusCreated, h.toDTO(r.Context(), c, false))
}

func (h *clientsHandler) get(w http.ResponseWriter, r *http.Request) {
	c, ok := h.load(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in clientInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := h.svc.Update(r.Context(), id, clients.Input{Name: in.Name, Remark: in.Remark, Email: strings.TrimSpace(in.Email)})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update client")
		return
	}
	h.audit(r, "client.update", c.ID, "{}")
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) del(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete client")
		return
	}
	h.audit(r, "client.delete", id, "{}")
	h.autoSync()
	w.WriteHeader(http.StatusNoContent)
}

func (h *clientsHandler) enable(w http.ResponseWriter, r *http.Request) { h.setEnabled(w, r, true) }
func (h *clientsHandler) disable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *clientsHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.SetEnabled(r.Context(), id, enabled)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update client")
		return
	}
	action := "client.disable"
	if enabled {
		action = "client.enable"
	}
	h.audit(r, action, id, "{}")
	h.autoSync()
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) setInbounds(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in clientInboundsInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := h.svc.SetInbounds(r.Context(), id, in.InboundIDs)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update grants")
		return
	}
	h.audit(r, "client.grants", id, `{"count":`+strconv.Itoa(len(in.InboundIDs))+`}`)
	h.autoSync()
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) rotateToken(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.RotateToken(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate token")
		return
	}
	h.audit(r, "client.rotate_token", id, "{}")
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) links(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	links, err := h.svc.Links(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build links")
		return
	}
	out := make([]clientLinkDTO, 0, len(links))
	for _, l := range links {
		out = append(out, clientLinkDTO{
			InboundID: l.InboundID, ServerName: l.ServerName, Protocol: l.Protocol,
			Remark: l.Remark, URI: l.URI,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// amneziawg returns the client's AmneziaWG configs (.conf + vpn://) for each
// granted AmneziaWG inbound.
func (h *clientsHandler) amneziawg(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	cfgs, err := h.svc.AmneziaWGConfigs(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build amneziawg configs")
		return
	}
	writeJSON(w, http.StatusOK, cfgs)
}

func (h *clientsHandler) qrcode(w http.ResponseWriter, r *http.Request) {
	c, ok := h.load(w, r)
	if !ok {
		return
	}
	target := r.URL.Query().Get("link")
	if target == "" {
		target = h.subURL(c.SubscriptionToken)
	}
	if target == "" {
		writeError(w, http.StatusBadRequest, "no link to encode")
		return
	}
	png, err := skipqr.Encode(target, skipqr.Medium, 512)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render qr")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *clientsHandler) config(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	links, err := h.svc.Links(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build config")
		return
	}
	uris := make([]string, 0, len(links))
	for _, l := range links {
		uris = append(uris, l.URI)
	}
	body := strings.Join(uris, "\n")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="subscription.txt"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// load resolves the {id} param to a client or writes the error response.
func (h *clientsHandler) load(w http.ResponseWriter, r *http.Request) (*store.Client, bool) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return nil, false
	}
	c, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client")
		return nil, false
	}
	return c, true
}
