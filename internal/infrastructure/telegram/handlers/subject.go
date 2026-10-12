package handlers

import (
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

type SubjectDict interface {
	Lookup(shortName string) (string, bool)
	Resolve(raw string) (cleaned string, fullName string, found bool)
	All() map[string]string
	Search(query string) []entities.SubjectInfo
}

func Subject(dict SubjectDict) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			return c.Send(
				"📚 *Справочник предметов*\n\nИспользование: `/subject <название>`\nНапример: `/subject ТОЭ` или `/subject КПиЯП`",
				tele.ModeMarkdown,
				keyboard.ReplyScheduleKeyboard,
			)
		}

		query := strings.Join(args, " ")
		cleaned, fullName, found := dict.Resolve(query)
		if found {
			return c.Send(
				fmt.Sprintf("📖 *%s*\n%s", cleaned, fullName),
				tele.ModeMarkdown,
				keyboard.ReplyScheduleKeyboard,
			)
		}

		// Поиск похожих совпадений
		similar := dict.Search(query)
		if len(similar) > 0 {
			var sb strings.Builder
			fmt.Fprintf(&sb, "Предмет *%s* не найден. Возможно, вы имели в виду:\n\n", query)
			limit := min(len(similar), 8)
			for i := range limit {
				fmt.Fprintf(&sb, "• *%s* — %s\n", similar[i].ShortName, similar[i].FullName)
			}
			return c.Send(sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}

		return c.Send(
			fmt.Sprintf("Предмет *%s* не найден в справочнике колледжа.", query),
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
		)
	}
}
