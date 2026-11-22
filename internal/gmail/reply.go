package gmail

import (
	"html"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

// ReplyRecipients works out To and Cc for a reply. Reply goes to the sender
// (or, when the user sent the message, back to its recipients). Reply all also
// copies everyone else on it, never the user.
func ReplyRecipients(m Message, self string, all bool) (to, cc string) {
	self = strings.ToLower(strings.TrimSpace(self))
	seen := map[string]bool{self: true}
	pick := func(fields ...string) []string {
		var out []string
		for _, f := range fields {
			list, err := mail.ParseAddressList(f)
			if err != nil {
				continue
			}
			for _, a := range list {
				k := strings.ToLower(a.Address)
				if seen[k] {
					continue
				}
				seen[k] = true
				out = append(out, a.String())
			}
		}
		return out
	}
	_, fromEmail := splitFrom(m.From)
	fromSelf := strings.EqualFold(fromEmail, self)
	var toList []string
	if fromSelf {
		toList = pick(m.To)
	} else {
		toList = pick(m.From)
	}
	if !all {
		return strings.Join(toList, ", "), ""
	}
	if !fromSelf {
		toList = append(toList, pick(m.To)...)
	}
	return strings.Join(toList, ", "), strings.Join(pick(m.Cc), ", ")
}

var tags = regexp.MustCompile(`(?s)<style.*?</style>|<script.*?</script>|<[^>]+>`)

// plainText is a message's text part, or its HTML part with the tags removed.
func plainText(m Message) string {
	if strings.TrimSpace(m.Text) != "" {
		return strings.ReplaceAll(m.Text, "\r\n", "\n")
	}
	t := tags.ReplaceAllString(strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n\n", "</div>", "\n").Replace(m.HTML), "")
	return strings.TrimSpace(html.UnescapeString(t))
}

func when(ms int64) string { return time.UnixMilli(ms).Format("Mon, Jan 2, 2006 at 3:04 PM") }

// ReplyDraft prefills a reply, reply all or forward. The original is quoted as
// text (and escaped inside a blockquote for the HTML part): its own markup is
// never carried into the compose window.
func ReplyDraft(m Message, mode, self string) Draft {
	text := plainText(m)
	subject := m.Subject
	d := Draft{ThreadID: m.ThreadID}
	if mode == "forward" {
		if !regexp.MustCompile(`(?i)^(fwd?|fw):`).MatchString(subject) {
			subject = "Fwd: " + subject
		}
		head := "---------- Forwarded message ---------\nFrom: " + m.From + "\nDate: " + when(m.Date) +
			"\nSubject: " + m.Subject + "\nTo: " + m.To
		if m.Cc != "" {
			head += "\nCc: " + m.Cc
		}
		d.Subject = subject
		d.Body = "\n\n" + head + "\n\n" + text
		d.HTML = "<br><br><div>" + strings.ReplaceAll(html.EscapeString(head), "\n", "<br>") + "</div><br>" +
			"<div>" + strings.ReplaceAll(html.EscapeString(text), "\n", "<br>") + "</div>"
		for _, a := range m.Attachments {
			if !a.Inline {
				d.Forwarded = append(d.Forwarded, a)
			}
		}
		return d
	}
	if !regexp.MustCompile(`(?i)^re:`).MatchString(subject) {
		subject = "Re: " + subject
	}
	d.Subject = subject
	d.To, d.Cc = ReplyRecipients(m, self, mode == "all")
	d.InReplyTo = m.MessageID
	d.References = strings.TrimSpace(m.References + " " + m.MessageID)
	intro := "On " + when(m.Date) + ", " + m.From + " wrote:"
	quoted := "> " + strings.ReplaceAll(text, "\n", "\n> ")
	d.Body = "\n\n" + intro + "\n" + quoted
	d.HTML = "<br><br><div>" + html.EscapeString(intro) + `</div><blockquote style="margin:0 0 0 .8ex;border-left:1px solid #ccc;padding-left:1ex">` +
		strings.ReplaceAll(html.EscapeString(text), "\n", "<br>") + "</blockquote>"
	return d
}

// Signature reads the default send-as address's signature (HTML), or "".
func Signature(client *http.Client) (string, error) {
	var r struct {
		SendAs []struct {
			IsDefault bool   `json:"isDefault"`
			IsPrimary bool   `json:"isPrimary"`
			Signature string `json:"signature"`
		} `json:"sendAs"`
	}
	if err := getJSON(client, api+"/settings/sendAs", &r); err != nil {
		return "", err
	}
	for _, s := range r.SendAs {
		if s.IsDefault {
			return s.Signature, nil
		}
	}
	for _, s := range r.SendAs {
		if s.IsPrimary {
			return s.Signature, nil
		}
	}
	return "", nil
}
