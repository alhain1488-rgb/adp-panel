// Package brand holds the panel's public name and the branding "plate" appended
// to every message the panel sends to users (Telegram and e-mail).
package brand

// Identity, shown bilingually (Russian, English in parentheses).
const (
	NameRU = "Абсолютно отвратительная панель"
	NameEN = "Absolutely Disgusting Panel"
	Emoji  = "🐰"
)

// TextFooter is the plain-text branding plate. Used in Telegram messages and as
// the plain-text alternative of e-mails.
func TextFooter() string {
	return Emoji + " " + NameRU + " (" + NameEN + ")"
}

// HTMLFooter renders the branding plate with the Cheremsha mascot (loaded from
// imgURL, e.g. https://panel.example/cheremsha.png). If imgURL is empty the
// emoji stands in for the image.
func HTMLFooter(imgURL string) string {
	var mark string
	if imgURL != "" {
		mark = `<img src="` + imgURL + `" alt="" width="32" height="32" ` +
			`style="vertical-align:middle;border-radius:6px;margin-right:8px">`
	} else {
		mark = Emoji + " "
	}
	return `<div style="margin-top:24px;padding-top:16px;border-top:1px solid #e5e5e5;` +
		`color:#666;font-size:13px;font-family:sans-serif">` +
		mark + `<b>` + NameRU + `</b> <span style="color:#999">(` + NameEN + `)</span></div>`
}
