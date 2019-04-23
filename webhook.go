package webhook

import (
	"bytes"
	"errors"
	"regexp"

	"github.com/requilence/integram"
)

var m = integram.HTMLRichText{}
var markdownRichText = integram.MarkdownRichText{}

type Config struct{
	integram.BotConfig
}

type webhook struct {
	Text        string
	Mrkdwn      bool
	Channel     string
	Attachments []struct {
		Pretext    string   `json:"pretext"`
		Fallback   string   `json:"fallback"`
		AuthorName string   `json:"author_name"`
		AuthorLink string   `json:"author_link"`
		Title      string   `json:"title"`
		TitleLink  string   `json:"title_link"`
		Text       string   `json:"text"`
		MrkdwnIn   []string `json:"mrkdwn_in"`
		ImageURL   string   `json:"image_url"`
		ThumbURL   string   `json:"thumb_url"`
		Ts         int      `json:"ts"`
	} `json:"attachments"`
}

// Service returns *integram.Service
func (c Config) Service() *integram.Service {
	return &integram.Service{
		Name:                "webhook",
		NameToPrint:         "Webhook",
		WebhookHandler:      webhookHandler,
		TGNewMessageHandler: update,
	}
}
func update(c *integram.Context) error {

	command, param := c.Message.GetCommand()

	if c.Message.IsEventBotAddedToGroup() {
		command = "start"
	}
	if param == "silent" {
		command = ""
	}

	switch command {

	case "start":
		return c.NewMessage().EnableAntiFlood().EnableHTML().
			SetText("Hi here! You can send " + m.URL("Slack-compatible", "https://api.slack.com/docs/message-formatting#message_formatting") + " simple webhooks to " + m.Bold("this chat") + " using this URL: \n" + m.Fixed(c.Chat.ServiceHookURL()) + "\n\nExample (JSON payload):\n" + m.Pre("{\"text\":\"So _advanced_\\nMuch *innovations* 🙀\"}")).Send()

	}
	return nil
}

func convertLinks(text string, regex *regexp.Regexp, encodeEntities func(string) string, linkFormat func(string, string) string) string {
	if encodeEntities == nil {
		encodeEntities = func(text string) string {
			return text
		}
	}
	submatches := regex.FindAllStringSubmatchIndex(text, -1)
	if submatches == nil {
		return encodeEntities(text)
	}

	convertedBuffer := new(bytes.Buffer)
	convertedBuffer.Grow(len(text))
	currentPosition := 0
	for _, submatch := range submatches {
		if submatch[0] > 0 {
			convertedBuffer.WriteString(encodeEntities(text[currentPosition:submatch[0]]))
		}
		if submatch[2] < 0 {
			// Code block, copy as-is
			convertedBuffer.WriteString(encodeEntities(text[submatch[0]:submatch[1]]))
		} else {
			// URL link, convert
			url := text[submatch[2]:submatch[3]]
			displayText := url
			if submatch[4] > 0 && submatch[4] != submatch[5] {
				displayText = text[submatch[4] + 1:submatch[5]]
			}
			convertedBuffer.WriteString(linkFormat(displayText, url))
		}
		currentPosition = submatch[1]
	}
	if currentPosition < len(text) {
		convertedBuffer.WriteString(encodeEntities(text[currentPosition:]))
	}
	return convertedBuffer.String()
}

func convertLinksToMarkdown(text string) string {
	// Escape URL links if outside code blocks.
	// Message format is documented at https://api.slack.com/docs/message-formatting)
	// References to a Slack channel (@), user (#) or variable (!) are kept as-is
	linkOrCodeBlockRegexp := regexp.MustCompile("```.+```|`[^`\n]+`|<([^@#! \n][^|> \n]*)(|[^>\n]+)?>")
	return convertLinks(text, linkOrCodeBlockRegexp, nil, markdownRichText.URL);
}

func convertLinksToHtml(text string) string {
	linkOrCodeBlockRegexp := regexp.MustCompile("<code>.*</code>|<pre>.*</pre>|<([^@#! \n][^|> \n]*)(|[^>\n]+)?>")
	return convertLinks(text, linkOrCodeBlockRegexp, nil, m.URL);
}

func convertPlainWithLinksToHTML(text string) string {
	linkRegexp := regexp.MustCompile("<([^@#! \n][^|> \n]*)(|[^>\n]+)?>")
	return convertLinks(text, linkRegexp, m.EncodeEntities, m.URL);
}

func webhookHandler(c *integram.Context, wc *integram.WebhookContext) (err error) {

	wh := webhook{Mrkdwn: true}
	err = wc.JSON(&wh)

	if err != nil {
		return
	}

	if len(wh.Attachments) > 0 {
		if wh.Text != "" {
			wh.Text += "\n"
		}
		text := ""

		if wh.Attachments[0].TitleLink != "" {
			wp := c.WebPreview(wh.Attachments[0].Title, wh.Attachments[0].AuthorName, wh.Attachments[0].Pretext, wh.Attachments[0].TitleLink, wh.Attachments[0].ThumbURL)
			text += m.URL(" ", wp) + " " + wh.Text
		}

		haveAttachmentWithText:=false
		haveMrkdwnAttachment:=false
		for i, attachment := range wh.Attachments {
			if i > 0 {
				text += "\n"
			}

			if attachment.Fallback != "" {
				attachment.Pretext = attachment.Fallback
			}

			if attachment.TitleLink != "" {
				text += m.URL(attachment.Title, attachment.TitleLink) + " "
			} else if attachment.Title != "" {
				text += m.Bold(attachment.Title) + " "
			} else if attachment.Pretext == "" {
				continue
			}

			haveAttachmentWithText = true
			for _, field := range attachment.MrkdwnIn {
				if field == "pretext" {
					haveMrkdwnAttachment = true
				}
			}

			text += attachment.Pretext
		}

		if haveAttachmentWithText {
			m := c.NewMessage().EnableAntiFlood()
			if haveMrkdwnAttachment {
				m.SetText(convertLinksToMarkdown(text)).EnableMarkdown()
			} else {
				m.SetText(convertLinksToHtml(text)).EnableHTML()
			}
			return m.Send()
		}
	}

	if wh.Text != "" {
		m := c.NewMessage().EnableAntiFlood()
		if wh.Mrkdwn {
			m.SetText(convertLinksToMarkdown(wh.Text + " " + wh.Channel)).EnableMarkdown()
		} else {
			m.SetText(convertPlainWithLinksToHTML(wh.Text + " " + wh.Channel)).EnableHTML()
		}
		return m.Send()
	}

	return errors.New("Text and Attachments not found")
}
