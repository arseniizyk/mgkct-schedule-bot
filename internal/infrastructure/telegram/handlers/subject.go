package handlers

import (
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

type SubjectDict interface {
	Lookup(shortName string) (string, bool)
	Resolve(raw string) (cleaned string, fullName string, found bool)
	All() map[string]string
	Search(query string) []entities.SubjectInfo
}

func Subject(dict SubjectDict, sm StateManager) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			if sm != nil {
				if err := sm.Set(c.Chat().ID, state.WaitingSubject); err != nil {
					return c.Send(messages.Internal)
				}
			}
			return c.Send(messages.WaitingSubject)
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
