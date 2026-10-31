package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
)

var (
	initOnce    sync.Once
	errInit     error
	regularFont *truetype.Font
	boldFont    *truetype.Font
	logoImage   image.Image

	faceTitle       font.Face
	faceDayHeading  font.Face
	faceTableHeader font.Face
	faceColNum      font.Face
	faceColSubj     font.Face
	faceColType     font.Face
	faceColRoom     font.Face
	faceColTeacher  font.Face
	faceFooter      font.Face
)

func initAssets() error {
	initOnce.Do(func() {
		regBytes, err := assetsFS.ReadFile("assets/regular.ttf")
		if err != nil {
			errInit = fmt.Errorf("read regular.ttf: %w", err)
			return
		}
		regularFont, err = truetype.Parse(regBytes)
		if err != nil {
			errInit = fmt.Errorf("parse regular.ttf: %w", err)
			return
		}

		boldBytes, err := assetsFS.ReadFile("assets/bold.ttf")
		if err != nil {
			errInit = fmt.Errorf("read bold.ttf: %w", err)
			return
		}
		boldFont, err = truetype.Parse(boldBytes)
		if err != nil {
			errInit = fmt.Errorf("parse bold.ttf: %w", err)
			return
		}

		logoBytes, err := assetsFS.ReadFile("assets/logo.png")
		if err != nil {
			errInit = fmt.Errorf("read logo.png: %w", err)
			return
		}
		srcLogo, _, err := image.Decode(bytes.NewReader(logoBytes))
		if err != nil {
			errInit = fmt.Errorf("decode logo.png: %w", err)
			return
		}

		// Масштабируем логотип высокого разрешения до 52x52 с CatmullRom сглаживанием
		scaled := image.NewRGBA(image.Rect(0, 0, 52, 52))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), srcLogo, srcLogo.Bounds(), draw.Over, nil)
		logoImage = scaled

		// Создаем шрифтовые гарнитуры для таблицы
		faceTitle = truetype.NewFace(boldFont, &truetype.Options{Size: 32, DPI: 72})
		faceDayHeading = truetype.NewFace(boldFont, &truetype.Options{Size: 15.5, DPI: 72})
		faceTableHeader = truetype.NewFace(boldFont, &truetype.Options{Size: 11, DPI: 72})
		faceColNum = truetype.NewFace(boldFont, &truetype.Options{Size: 12, DPI: 72})
		faceColSubj = truetype.NewFace(regularFont, &truetype.Options{Size: 11.5, DPI: 72})
		faceColType = truetype.NewFace(regularFont, &truetype.Options{Size: 10.5, DPI: 72})
		faceColRoom = truetype.NewFace(boldFont, &truetype.Options{Size: 11.5, DPI: 72})
		faceColTeacher = truetype.NewFace(regularFont, &truetype.Options{Size: 11, DPI: 72})
		faceFooter = truetype.NewFace(regularFont, &truetype.Options{Size: 10, DPI: 72})
	})

	return errInit
}

// Константы геометрии холста
const (
	canvasWidth     = 980
	leftMargin      = 10
	columnGap       = 16
	tableWidth      = 472
	gridRowGap      = 20
	headerHeight    = 58
	topMargin       = 14
	afterHeaderGap  = 14
	headingHeight   = 19
	headingGap      = 8
	tableTheadH     = 25
	rowSingleH      = 30
	rowDoubleH      = 48
	footerGap       = 18
	footerH         = 16
	bottomCanvasGap = 14
)

// Ширины колонок таблицы: № (29), Предмет (144), Вид (58), Аудитория (96), Преподаватель (145) = 472px
var colWidths = []float64{29, 144, 58, 96, 145}

// Цветовая палитра
var (
	colorBg          = color.RGBA{R: 255, G: 255, B: 255, A: 255} // #ffffff
	colorBorder      = color.RGBA{R: 220, G: 220, B: 220, A: 255} // #dcdcdc
	colorTheadBg     = color.RGBA{R: 229, G: 231, B: 235, A: 255} // #e5e7eb
	colorNumBg       = color.RGBA{R: 247, G: 247, B: 247, A: 255} // #f7f7f7
	colorTextBlack   = color.RGBA{R: 0, G: 0, B: 0, A: 255}       // #000000
	colorTextType    = color.RGBA{R: 51, G: 51, B: 51, A: 255}    // #333333
	colorTextTeacher = color.RGBA{R: 34, G: 34, B: 34, A: 255}    // #222222
	colorTextFooter  = color.RGBA{R: 119, G: 119, B: 119, A: 255} // #777777
)

type pairRowData struct {
	num       int
	isDouble  bool
	pair1     *entities.Pair
	pair2     *entities.Pair
	rowHeight float64
}

type dayData struct {
	title       string
	rows        []pairRowData
	tableHeight float64
	totalHeight float64
}

// RenderGroupWeek генерирует PNG-изображение расписания недели для учебной группы
func RenderGroupWeek(group *entities.Group, subgroup int) ([]byte, error) {
	const op = "renderer.RenderGroupWeek"

	if err := initAssets(); err != nil {
		return nil, fmt.Errorf("%s: init assets: %w", op, err)
	}

	if group == nil {
		return nil, fmt.Errorf("%s: group is nil", op)
	}

	filteredGroup := group
	if subgroup > 0 {
		filteredGroup = formatter.FilterGroupForSubgroup(group, subgroup)
	}

	groupTitle := fmt.Sprintf("Группа - %d", filteredGroup.ID)
	if subgroup > 0 {
		groupTitle = fmt.Sprintf("Группа - %d (%d п/г)", filteredGroup.ID, subgroup)
	}

	daysData := prepareDaysData(filteredGroup.Days)
	imgBytes, err := renderWeekCanvas(groupTitle, daysData, "Преподаватель")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return imgBytes, nil
}

// RenderTeacherWeek генерирует PNG-изображение расписания недели для преподавателя
func RenderTeacherWeek(teacher *entities.Teacher) ([]byte, error) {
	const op = "renderer.RenderTeacherWeek"

	if err := initAssets(); err != nil {
		return nil, fmt.Errorf("%s: init assets: %w", op, err)
	}

	if teacher == nil {
		return nil, fmt.Errorf("%s: teacher is nil", op)
	}

	teacherTitle := fmt.Sprintf("Преподаватель - %s", teacher.Name)
	daysData := prepareDaysData(teacher.Days)
	imgBytes, err := renderWeekCanvas(teacherTitle, daysData, "Группа")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return imgBytes, nil
}

// prepareDaysData подготавливает данные для 6 учебных дней недели (Пн..Сб)
func prepareDaysData(days []*entities.Day) []dayData {
	daysData := make([]dayData, 6)
	defaultDayNames := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

	for i := range 6 {
		var day *entities.Day
		if i < len(days) && days[i] != nil {
			day = days[i]
		}

		title := defaultDayNames[i]
		if day != nil && strings.TrimSpace(day.Name) != "" {
			name := strings.ReplaceAll(day.Name, " | ", ", ")
			title = strings.TrimSpace(name)
		}

		// Стандартный день: 4 пары (1..4).
		// Если в дне есть пары дальше 4-й (например, 5-я или 6-я), расширяем таблицу до последней непустой пары.
		numPairs := 4
		if lastIdx := findLastLessonIndex(day); lastIdx >= 4 {
			numPairs = lastIdx + 1
		}

		rows := make([]pairRowData, numPairs)
		tableH := float64(tableTheadH)

		for pIdx := range numPairs {
			pNum := pIdx + 1
			var p1, p2 *entities.Pair
			isDbl := false

			if day != nil && pIdx < len(day.Subjects) && day.Subjects[pIdx] != nil && !day.Subjects[pIdx].IsEmpty {
				subj := day.Subjects[pIdx]
				nonEmpty := make([]*entities.Pair, 0, len(subj.Pairs))
				for _, p := range subj.Pairs {
					if p != nil && (strings.TrimSpace(p.Name) != "" || strings.TrimSpace(p.Group) != "") {
						nonEmpty = append(nonEmpty, p)
					}
				}

				if len(nonEmpty) >= 2 {
					isDbl = true
					p1 = nonEmpty[0]
					p2 = nonEmpty[1]
				} else if len(nonEmpty) == 1 {
					p1 = nonEmpty[0]
				}
			}

			rH := float64(rowSingleH)
			if isDbl {
				rH = float64(rowDoubleH)
			}

			rows[pIdx] = pairRowData{
				num:       pNum,
				isDouble:  isDbl,
				pair1:     p1,
				pair2:     p2,
				rowHeight: rH,
			}
			tableH += rH
		}

		totH := float64(headingHeight+headingGap) + tableH
		daysData[i] = dayData{
			title:       title,
			rows:        rows,
			tableHeight: tableH,
			totalHeight: totH,
		}
	}

	return daysData
}

// renderWeekCanvas выполняет сборку графического холста недели с заголовком, сеткой дней и подвалом
func renderWeekCanvas(title string, daysData []dayData, col4Title string) ([]byte, error) {
	// Расчет динамической высоты строк сетки 2x3
	gridRowHeights := make([]float64, 3)
	for r := range 3 {
		hLeft := daysData[2*r].totalHeight
		hRight := daysData[2*r+1].totalHeight
		if hLeft >= hRight {
			gridRowHeights[r] = hLeft
		} else {
			gridRowHeights[r] = hRight
		}
	}

	gridStartY := float64(topMargin + headerHeight + afterHeaderGap) // 14 + 58 + 14 = 86
	row0Y := gridStartY
	row1Y := row0Y + gridRowHeights[0] + float64(gridRowGap)
	row2Y := row1Y + gridRowHeights[1] + float64(gridRowGap)
	tablesBottomY := row2Y + gridRowHeights[2]

	footerY := tablesBottomY + float64(footerGap)
	totalCanvasHeight := int(footerY + float64(footerH+bottomCanvasGap))

	dc := gg.NewContext(canvasWidth, totalCanvasHeight)
	dc.SetColor(colorBg)
	dc.Clear()

	// Шапка (Логотип MGKCT + Название группы / ФИО преподавателя)
	dc.DrawImage(logoImage, 14, topMargin+(headerHeight-52)/2)

	dc.SetFontFace(faceTitle)
	dc.SetColor(colorTextBlack)
	displayTitle := truncateText(dc, title, canvasWidth-140)
	dc.DrawStringAnchored(displayTitle, canvasWidth/2.0, float64(topMargin)+float64(headerHeight)/2.0, 0.5, 0.5)

	// Отрисовка 6 таблиц дней
	gridRowStarts := []float64{row0Y, row1Y, row2Y}
	for i := range 6 {
		colIdx := i % 2
		rowIdx := i / 2

		boxX := float64(leftMargin)
		if colIdx == 1 {
			boxX = float64(leftMargin + tableWidth + columnGap)
		}
		boxY := gridRowStarts[rowIdx]

		drawDayTable(dc, boxX, boxY, daysData[i], col4Title)
	}

	// Подвал (Footer)
	dc.SetFontFace(faceFooter)
	dc.SetColor(colorTextFooter)
	footerLeft := "@mgkct_bot"
	footerRight := fmt.Sprintf("Сгенерировано в: %s", time.Now().Format("02.01.2006 15:04:05"))

	dc.DrawStringAnchored(footerLeft, 14, footerY+float64(footerH)/2.0, 0, 0.5)
	dc.DrawStringAnchored(footerRight, canvasWidth-14, footerY+float64(footerH)/2.0, 1.0, 0.5)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}

	return buf.Bytes(), nil
}

func drawDayTable(dc *gg.Context, boxX, boxY float64, d dayData, col4Title string) {
	// Заголовок дня (День, Дата)
	dc.SetFontFace(faceDayHeading)
	dc.SetColor(colorTextBlack)
	dc.DrawStringAnchored(d.title, boxX+tableWidth/2.0, boxY+float64(headingHeight)/2.0, 0.5, 0.5)

	tableY := boxY + float64(headingHeight+headingGap)

	// Координаты колонок X
	xCol0 := boxX
	xCol1 := xCol0 + colWidths[0] // 29
	xCol2 := xCol1 + colWidths[1] // 144
	xCol3 := xCol2 + colWidths[2] // 58
	xCol4 := xCol3 + colWidths[3] // 96
	xEnd := xCol4 + colWidths[4]  // 145 = 472

	// Шапка таблицы: заливка #e5e7eb
	dc.SetColor(colorTheadBg)
	dc.DrawRectangle(boxX, tableY, tableWidth, float64(tableTheadH))
	dc.Fill()

	// Тексты шапки таблицы (все центрированы)
	dc.SetFontFace(faceTableHeader)
	dc.SetColor(colorTextBlack)
	thY := tableY + float64(tableTheadH)/2.0

	dc.DrawStringAnchored("№", xCol0+colWidths[0]/2.0, thY, 0.5, 0.5)
	dc.DrawStringAnchored("Предмет", xCol1+colWidths[1]/2.0, thY, 0.5, 0.5)
	dc.DrawStringAnchored("Вид", xCol2+colWidths[2]/2.0, thY, 0.5, 0.5)
	dc.DrawStringAnchored("Аудитория", xCol3+colWidths[3]/2.0, thY, 0.5, 0.5)
	dc.DrawStringAnchored(col4Title, xCol4+colWidths[4]/2.0, thY, 0.5, 0.5)

	// Строки пар
	currY := tableY + float64(tableTheadH)

	for _, r := range d.rows {
		// Заливка колонки № фоном #f7f7f7
		dc.SetColor(colorNumBg)
		dc.DrawRectangle(xCol0, currY, colWidths[0], r.rowHeight)
		dc.Fill()

		// Номер пары: центрирован
		dc.SetFontFace(faceColNum)
		dc.SetColor(colorTextBlack)
		dc.DrawStringAnchored(strconv.Itoa(r.num), xCol0+colWidths[0]/2.0, currY+r.rowHeight/2.0, 0.5, 0.5)

		if !r.isDouble {
			// Одинарная строка (30px)
			centerY := currY + r.rowHeight/2.0
			if r.pair1 != nil {
				drawLessonCell(dc, r.pair1, xCol1, xCol2, xCol3, xCol4, centerY)
			}
		} else {
			// Двойная строка подгрупп (48px)
			subH := r.rowHeight / 2.0 // 24px
			center1Y := currY + subH/2.0
			center2Y := currY + subH + subH/2.0

			if r.pair1 != nil {
				drawLessonCell(dc, r.pair1, xCol1, xCol2, xCol3, xCol4, center1Y)
			}
			if r.pair2 != nil {
				drawLessonCell(dc, r.pair2, xCol1, xCol2, xCol3, xCol4, center2Y)
			}

			// Пунктирный разделитель подгрупп между подстроками
			dc.SetColor(colorBorder)
			dc.SetLineWidth(1.0)
			dc.SetDash(3, 3)
			dividerY := currY + subH
			dc.DrawLine(xCol1+0.5, dividerY+0.5, xEnd-0.5, dividerY+0.5)
			dc.Stroke()
			dc.SetDash() // Сброс штриховки
		}

		currY += r.rowHeight
	}

	// 6. Отрисовка всех линий сетки таблицы (1px solid #dcdcdc)
	dc.SetColor(colorBorder)
	dc.SetLineWidth(1.0)

	// Внешняя граница таблицы
	dc.DrawRectangle(boxX+0.5, tableY+0.5, tableWidth-1.0, d.tableHeight-1.0)
	dc.Stroke()

	// Внутренние горизонтальные линии
	lineY := tableY + float64(tableTheadH)
	for i, r := range d.rows {
		dc.DrawLine(boxX+0.5, lineY+0.5, xEnd-0.5, lineY+0.5)
		dc.Stroke()
		if i < len(d.rows)-1 {
			lineY += r.rowHeight
		}
	}

	// Внутренние вертикальные линии
	colDividers := []float64{xCol1, xCol2, xCol3, xCol4}
	for _, x := range colDividers {
		dc.DrawLine(x+0.5, tableY+0.5, x+0.5, tableY+d.tableHeight-0.5)
		dc.Stroke()
	}
}

func drawLessonCell(dc *gg.Context, p *entities.Pair, xSubj, xType, xRoom, xTeacher, centerY float64) {
	// 1. Предмет: выравнивание по левому краю с отступом 6px
	dc.SetFontFace(faceColSubj)
	dc.SetColor(colorTextBlack)
	subjText := truncateText(dc, p.Name, colWidths[1]-10.0)
	dc.DrawStringAnchored(subjText, xSubj+6.0, centerY, 0.0, 0.5)

	// 2. Вид занятия: строго по центру
	dc.SetFontFace(faceColType)
	dc.SetColor(colorTextType)
	typeText := truncateText(dc, p.Type, colWidths[2]-4.0)
	dc.DrawStringAnchored(typeText, xType+colWidths[2]/2.0, centerY, 0.5, 0.5)

	// 3. Аудитория: СТРОГО ПО ЦЕНТРУ!
	dc.SetFontFace(faceColRoom)
	dc.SetColor(colorTextBlack)
	roomText := strings.TrimSpace(p.Class)
	if roomText == "" {
		roomText = "-"
	}
	roomText = truncateText(dc, roomText, colWidths[3]-6.0)
	dc.DrawStringAnchored(roomText, xRoom+colWidths[3]/2.0, centerY, 0.5, 0.5)

	// 4. Преподаватель: выравнивание по левому краю с отступом 6px
	dc.SetFontFace(faceColTeacher)
	dc.SetColor(colorTextTeacher)
	teacherText := truncateText(dc, p.Group, colWidths[4]-10.0)
	dc.DrawStringAnchored(teacherText, xTeacher+6.0, centerY, 0.0, 0.5)
}

func truncateText(dc *gg.Context, text string, maxWidth float64) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	w, _ := dc.MeasureString(text)
	if w <= maxWidth {
		return text
	}

	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		cand := strings.TrimSpace(string(runes)) + "..."
		cw, _ := dc.MeasureString(cand)
		if cw <= maxWidth {
			return cand
		}
	}

	return text
}

// findLastLessonIndex находит индекс (0-based) последней пары в дне, содержащей занятия.
// Возвращает -1, если в дне нет занятий.
func findLastLessonIndex(day *entities.Day) int {
	if day == nil || len(day.Subjects) == 0 {
		return -1
	}

	for idx, subj := range slices.Backward(day.Subjects) {
		if subj == nil || subj.IsEmpty {
			continue
		}
		for _, p := range subj.Pairs {
			if p != nil && (strings.TrimSpace(p.Name) != "" || strings.TrimSpace(p.Group) != "" || strings.TrimSpace(p.Class) != "") {
				return idx
			}
		}
	}

	return -1
}
