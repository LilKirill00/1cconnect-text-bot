package bot

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"connect-text-bot/bot/messages"
	"connect-text-bot/bot/requests"
	"connect-text-bot/botconfig_parser"
	"connect-text-bot/config"
	"connect-text-bot/database"
	"connect-text-bot/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Receive(c *gin.Context) {
	var msg messages.Message
	if err := c.BindJSON(&msg); err != nil {
		logger.Warning("Error while receive message", err)

		c.Status(http.StatusBadRequest)
		return
	}

	logger.Debug("Receive message:", msg)

	// Реагируем только на сообщения пользователя
	if (msg.MessageType == messages.MESSAGE_TEXT || msg.MessageType == messages.MESSAGE_FILE) && msg.MessageAuthor != nil && msg.UserId != *msg.MessageAuthor {
		c.Status(http.StatusOK)
		return
	}

	cCp := c.Copy()
	go func(cCp *gin.Context, msg messages.Message) {
		chatState := msg.GetState(c)

		newState, err := processMessage(c, &msg, &chatState)
		if err != nil {
			logger.Warning("Error processMessage", err)
		}

		err = msg.ChangeCacheState(c, &chatState, newState)
		if err != nil {
			logger.Warning("Error changeState", err)
		}
	}(cCp, msg)

	c.Status(http.StatusOK)
}

// заполнить шаблон данными
func fillTemplateWithInfo(c *gin.Context, msg *messages.Message, text string) (result string, err error) {
	// проверяем есть ли шаблон в тексте чтобы лишний раз не выполнять обработку
	if !strings.Contains(text, "{{") && !strings.Contains(text, "}}") {
		return text, nil
	}

	state := msg.GetState(c)

	// формируем шаблон
	templ, err := template.New("cmd").Parse(text)
	if err != nil {
		return
	}

	// создаем объединенные данные
	combinedData := struct {
		User   requests.User
		Var    map[string]string
		Ticket database.Ticket
	}{
		User:   state.User,
		Var:    state.Vars,
		Ticket: state.Ticket,
	}

	// заполняем шаблон
	var templOutput bytes.Buffer
	err = templ.Execute(&templOutput, combinedData)
	if err != nil {
		return
	}

	return templOutput.String(), err
}

// getFileNames - Получить список файлов из папки files.
func getFileNames(root string) map[string]bool {
	files := make(map[string]bool)
	root, _ = filepath.Abs(root)
	filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			files[path] = true
		}
		return nil
	})
	return files
}

func IsImage(file string) bool {
	reImg, _ := regexp.Compile(`(?i)\.(png|jpg|jpeg|bmp)$`)

	return reImg.MatchString(file)
}

func getFileInfo(filename, filesDir string) (isImage bool, filePath string, err error) {
	isImage = IsImage(filename)

	fileNames := getFileNames(filesDir)
	fullName, _ := filepath.Abs(filepath.Join(filesDir, filename))
	if !fileNames[fullName] {
		err = fmt.Errorf("не удалось найти и отправить файл: %s", filename)
		logger.Info(err)
	}

	filePath, _ = filepath.Abs(filepath.Join(filesDir, filename))
	return
}

func SendAnswer(c *gin.Context, msg *messages.Message, chatState *messages.Chat, menu *botconfig_parser.Levels, goTo, filesDir string, err error) (string, error) {
	var toSend *[][]requests.KeyboardKey

	for i := 0; i < len(menu.Menu[goTo].Answer); i++ {
		// Отправляем клаву только с последним сообщением.
		// Т.к в дп4 криво отображается.
		if i == len(menu.Menu[goTo].Answer)-1 {
			toSend = menu.GenKeyboard(goTo)
		}
		if menu.Menu[goTo].Answer[i].Chat != "" {
			r, _ := fillTemplateWithInfo(c, msg, menu.Menu[goTo].Answer[i].Chat)
			msg.Send(c, r, toSend)
		}
		if menu.Menu[goTo].Answer[i].File != "" {
			if isImage, filePath, err := getFileInfo(menu.Menu[goTo].Answer[i].File, filesDir); err == nil {
				msg.SendFile(c, isImage, menu.Menu[goTo].Answer[i].File, filePath, &menu.Menu[goTo].Answer[i].FileText, toSend)
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	if err == nil && menu.Menu[goTo].DoButton != nil {
		err := msg.ChangeCacheSavedButton(c, chatState, menu.Menu[goTo].DoButton)
		if err != nil {
			return finalSend(c, msg, chatState, "", err)
		}

		// выполнить действие кнопки
		err = msg.ChangeCacheState(c, chatState, database.START)
		if err != nil {
			return finalSend(c, msg, chatState, "", err)
		}
		return processMessage(c, msg, chatState)
	}
	return goTo, err
}

// переход на следующую стадию формирования заявки
func nextStageTicketButton(c *gin.Context, msg *messages.Message, chatState *messages.Chat, button *botconfig_parser.TicketButton, nextVar string) (string, error) {
	goTo := database.CREATE_TICKET
	ticket := database.Ticket{}
	text := ""

	// настройки для клавиатуры
	keyboard := &[][]requests.KeyboardKey{}
	btnAgain := []requests.KeyboardKey{{Id: "1", Text: "Далее"}}
	btnBack := []requests.KeyboardKey{{Id: "2", Text: "Назад"}}
	btnCancel := []requests.KeyboardKey{{Id: "0", Text: "Отмена"}}
	btnConfirm := []requests.KeyboardKey{{Id: "1", Text: "Подтверждаю"}}

	// проверяем на какое следующее меню надо отправить
	if nextVar == ticket.GetTheme() {
		text = button.Data.Theme.Text
		if button.Data.Theme.DefaultValue != nil {
			// подставляем данные если value содержит шаблон
			defaultValue, err := fillTemplateWithInfo(c, msg, *button.Data.Theme.DefaultValue)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// присвоить значение по умолчанию
			err = msg.ChangeCacheTicket(c, chatState, nextVar, []string{defaultValue})
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// переходим к следующее шагу
			nextVar = ticket.GetDescription()
		} else {
			// формируем клавиатуру
			*keyboard = append(*keyboard, btnAgain)
			*keyboard = append(*keyboard, btnBack)
			*keyboard = append(*keyboard, btnCancel)
		}
	}
	if nextVar == ticket.GetDescription() {
		text = button.Data.Description.Text
		if button.Data.Description.DefaultValue != nil {
			// подставляем данные если value содержит шаблон
			defaultValue, err := fillTemplateWithInfo(c, msg, *button.Data.Description.DefaultValue)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// присвоить значение по умолчанию
			err = msg.ChangeCacheTicket(c, chatState, nextVar, []string{defaultValue})
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// переходим к следующее шагу
			nextVar = ticket.GetExecutor()
		} else {
			// формируем клавиатуру
			*keyboard = append(*keyboard, btnAgain)
			*keyboard = append(*keyboard, btnBack)
			*keyboard = append(*keyboard, btnCancel)
		}
	}
	if nextVar == ticket.GetExecutor() {
		text = button.Data.Executor.Text
		if button.Data.Executor.DefaultValue != nil {
			r, err := msg.GetSpecialist(c, uuid.MustParse(*button.Data.Executor.DefaultValue))
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}
			fio := strings.TrimSpace(fmt.Sprintf("%s %s %s", r.Surname, r.Name, r.Patronymic))

			// присвоить значение по умолчанию
			err = msg.ChangeCacheTicket(c, chatState, nextVar, []string{r.UserId.String(), fio})
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// переходим к следующее шагу
			nextVar = ticket.GetService()
		} else {
			// получаем список специалистов
			listSpecs, err := msg.GetSpecialists(c, msg.LineId)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// формируем клавиатуру
			for _, v := range listSpecs {
				*keyboard = append(*keyboard, []requests.KeyboardKey{{Text: fmt.Sprintf("%s %s %s", v.Surname, v.Name, v.Patronymic)}})
			}
			*keyboard = append(*keyboard, btnAgain)
			*keyboard = append(*keyboard, btnBack)
			*keyboard = append(*keyboard, btnCancel)
		}
	}
	var ticketData *requests.GetTicketDataResponse = nil
	if nextVar == ticket.GetService() {
		text = button.Data.Service.Text
		if button.Data.Service.DefaultValue != nil {
			tr, err := msg.GetTicketData(c)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}
			ticketData = &tr
			kinds, err := msg.GetTicketDataKinds(c, ticketData)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// присвоить значение по умолчанию
			isFind := false
			for _, k := range kinds {
				if k.ID.String() == *button.Data.Service.DefaultValue {
					err = msg.ChangeCacheTicket(c, chatState, nextVar, []string{k.ID.String(), k.Name})
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}
					isFind = true
					break
				}
			}
			if !isFind {
				return finalSend(c, msg, chatState, "", errors.New("указанное значение по умолчанию (value) невозможно применить в (service) по текущей линии"))
			}

			// переходим к следующее шагу
			nextVar = ticket.GetServiceType()
		} else {
			// формируем клавиатуру
			kinds, err := msg.GetTicketDataKinds(c, nil)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}
			for _, v := range kinds {
				*keyboard = append(*keyboard, []requests.KeyboardKey{{Text: v.Name}})
			}
			*keyboard = append(*keyboard, btnAgain)
			*keyboard = append(*keyboard, btnBack)
			*keyboard = append(*keyboard, btnCancel)
		}
	}
	if nextVar == ticket.GetServiceType() {
		text = button.Data.ServiceType.Text
		if button.Data.ServiceType.DefaultValue != nil {
			types, err := msg.GetTicketDataTypesWhereKind(c, ticketData, chatState.Ticket.Service.Id)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}

			// присвоить значение по умолчанию
			isFind := false
			for _, t := range types {
				if t.ID.String() == *button.Data.ServiceType.DefaultValue {
					err = msg.ChangeCacheTicket(c, chatState, nextVar, []string{t.ID.String(), t.Name})
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}
					isFind = true
					break
				}
			}
			if !isFind {
				return finalSend(c, msg, chatState, "", errors.New("указанное значение по умолчанию (value) невозможно применить в (type) для выбранного (service)"))
			}

			// переходим к следующее шагу
			nextVar = "FINAL"
		} else {
			// формируем клавиатуру
			kindTypes, err := msg.GetTicketDataTypesWhereKind(c, nil, chatState.Ticket.Service.Id)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}
			for _, v := range kindTypes {
				*keyboard = append(*keyboard, []requests.KeyboardKey{{Text: v.Name}})
			}
			*keyboard = append(*keyboard, btnAgain)
			*keyboard = append(*keyboard, btnBack)
			*keyboard = append(*keyboard, btnCancel)
		}
	}
	if nextVar == "FINAL" {
		text = button.TicketInfo
		*keyboard = append(*keyboard, btnConfirm)
		*keyboard = append(*keyboard, btnBack)
		*keyboard = append(*keyboard, btnCancel)
	}

	// формируем сообщение
	r, err := fillTemplateWithInfo(c, msg, text)
	if err != nil {
		return finalSend(c, msg, chatState, "", err)
	}

	// сохраняем имя переменной куда будем записывать результат
	err = msg.ChangeCacheVars(c, chatState, database.VAR_FOR_SAVE, nextVar)
	if err != nil {
		return finalSend(c, msg, chatState, "", err)
	}

	err = msg.Send(c, r, keyboard)
	return goTo, err
}

// возврат на предыдущий шаг формирования заявки
func prevStageTicketButton(c *gin.Context, msg *messages.Message, chatState *messages.Chat, button *botconfig_parser.TicketButton, currentVar string) (string, error) {
	t := database.Ticket{}

	if currentVar == "FINAL" {
		currentVar = t.GetServiceType()
		if button.Data.ServiceType.DefaultValue == nil {
			return nextStageTicketButton(c, msg, chatState, button, currentVar)
		}
	}
	if currentVar == t.GetServiceType() {
		currentVar = t.GetService()
		if button.Data.Service.DefaultValue == nil {
			return nextStageTicketButton(c, msg, chatState, button, currentVar)
		}
	}
	if currentVar == t.GetService() {
		currentVar = t.GetExecutor()
		if button.Data.Executor.DefaultValue == nil {
			return nextStageTicketButton(c, msg, chatState, button, currentVar)
		}
	}
	if currentVar == t.GetExecutor() {
		currentVar = t.GetDescription()
		if button.Data.Description.DefaultValue == nil {
			return nextStageTicketButton(c, msg, chatState, button, currentVar)
		}
	}
	if currentVar == t.GetDescription() {
		currentVar = t.GetTheme()
		if button.Data.Theme.DefaultValue == nil {
			return nextStageTicketButton(c, msg, chatState, button, currentVar)
		}
	}
	if currentVar == t.GetDescription() {
		currentVar = t.GetTheme()
		if button.Data.Theme.DefaultValue == nil {
			return nextStageTicketButton(c, msg, chatState, button, currentVar)
		}
	}
	if currentVar == t.GetTheme() {
		state := msg.GetState(c)
		cnf := c.MustGet("cnf").(*config.Conf)
		menu := c.MustGet("menus").(*botconfig_parser.Levels)

		// чистим данные
		err := msg.ClearCacheOmitemptyFields(c, chatState)
		if err != nil {
			return finalSend(c, msg, chatState, "", err)
		}

		return SendAnswer(c, msg, chatState, menu, state.PreviousState, cnf.FilesDir, err)
	}

	return finalSend(c, msg, chatState, "", errors.New("не найдено куда направить пользователя по кнопке Назад"))
}

// Проверить нажата ли BackButton
func getGoToIfClickedBackBtn(btn *botconfig_parser.Button, state messages.Chat) (goTo string) {
	if btn != nil && btn.BackButton {
		if state.PreviousState != database.GREETINGS {
			goTo = state.PreviousState
		} else {
			goTo = database.START
		}
	}
	return
}

func processMessage(c *gin.Context, msg *messages.Message, chatState *messages.Chat) (string, error) {
	cnf := c.MustGet("cnf").(*config.Conf)
	menu := c.MustGet("menus").(*botconfig_parser.Levels)
	time.Sleep(250 * time.Millisecond)

	var err error

	switch msg.MessageType {
	// Первый запуск.
	case messages.MESSAGE_TREATMENT_START_BY_USER:
		if chatState.CurrentState == database.START {
			return chatState.CurrentState, nil
		}
		return database.GREETINGS, nil

	// Нажатие меню ИЛИ Любое сообщение (текст, файл, попытка звонка).
	case messages.MESSAGE_CALL_START_TREATMENT,
		messages.MESSAGE_CALL_START_NO_TREATMENT,
		messages.MESSAGE_TREATMENT_START_BY_SPEC,
		messages.MESSAGE_TREATMENT_CLOSE,
		messages.MESSAGE_TREATMENT_CLOSE_ACTIVE:
		err = msg.Start(cnf)
		return database.GREETINGS, err

	case messages.MESSAGE_NO_FREE_SPECIALISTS:
		err = msg.RerouteTreatment(c)
		return database.GREETINGS, err

	// Пользователь отправил сообщение.
	case messages.MESSAGE_TEXT,
		messages.MESSAGE_FILE:
		text := strings.ToLower(strings.TrimSpace(msg.Text))

		switch chatState.CurrentState {

		case database.GREETINGS:
			switch text {
			case "меню", "menu":
				// Ходим дальше
			default:
				currentMenu := database.START
				if cm, ok := menu.Menu[currentMenu]; ok {
					if !cm.QnaDisable && menu.UseQNA.Enabled {
						// logger.Info("QNA", msg, chatState)
						qnaText, isClose, request_id, result_id := getMessageFromQNA(msg, cnf)
						if qnaText != "" {
							// Была подсказка
							go msg.QnaSelected(cnf, request_id, result_id)

							if isClose {
								err = msg.Send(c, qnaText, nil)
								msg.CloseTreatment(c)
								return currentMenu, err
							}

							err = msg.Send(c, qnaText, menu.GenKeyboard(currentMenu))
							return currentMenu, err
						}

						return SendAnswer(c, msg, chatState, menu, database.FAIL_QNA, cnf.FilesDir, err)
					}
				}
			}

			if menu.FirstGreeting {
				msg.Send(c, menu.GreetingMessage, nil)
				time.Sleep(time.Second)
			}
			return SendAnswer(c, msg, chatState, menu, database.START, cnf.FilesDir, err)

		// пользователь попадет сюда в случае регистрации заявки
		case database.CREATE_TICKET:
			state := msg.GetState(c)
			btn := menu.GetButton(state.CurrentState, text)
			tBtn := state.SavedButton.TicketButton
			ticket := database.Ticket{}

			// переходим если нажата BackButton
			goTo := getGoToIfClickedBackBtn(btn, state)
			if goTo != "" {
				// перейти в определенное меню если настроен параметр goto
				if tBtn.Goto != "" {
					goTo = tBtn.Goto
				}

				// чистим данные
				err = msg.ClearCacheOmitemptyFields(c, chatState)
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}

				return SendAnswer(c, msg, chatState, menu, goTo, cnf.FilesDir, err)
			}

			// узнаем имя переменной
			varName, exist := msg.GetCacheVar(c, database.VAR_FOR_SAVE)
			if !exist {
				return finalSend(c, msg, chatState, "", err)
			}

			// проверяем нажата ли кнопка Назад
			if btn != nil && btn.Goto == database.CREATE_TICKET_PREV_STAGE {
				return prevStageTicketButton(c, msg, chatState, tBtn, varName)
			}

			switch varName {
			case ticket.GetTheme():
				textForSave := msg.Text
				// если кнопка перехода к следующему шагу
				if btn != nil && btn.Goto == database.CREATE_TICKET {
					textForSave = ""
				}
				err = msg.ChangeCacheTicket(c, chatState, varName, []string{textForSave})
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}

				return nextStageTicketButton(c, msg, chatState, tBtn, ticket.GetDescription())

			case ticket.GetDescription():
				textForSave := msg.Text
				// если кнопка перехода к следующему шагу
				if btn != nil && btn.Goto == database.CREATE_TICKET {
					textForSave = ""
				}
				err = msg.ChangeCacheTicket(c, chatState, varName, []string{textForSave})
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}

				return nextStageTicketButton(c, msg, chatState, tBtn, ticket.GetExecutor())

			case ticket.GetExecutor():
				// получаем список специалистов
				listSpecs, err := msg.GetSpecialists(c, msg.LineId)
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}

				// если кнопка перехода к следующему шагу
				if btn != nil && btn.Goto == database.CREATE_TICKET {
					err = msg.Send(c, "Данный этап нельзя пропустить", nil)
					return database.CREATE_TICKET, err
				} else {
					for _, v := range listSpecs {
						fio := strings.TrimSpace(fmt.Sprintf("%s %s %s", v.Surname, v.Name, v.Patronymic))
						if msg.Text == fio {
							err = msg.ChangeCacheTicket(c, chatState, varName, []string{v.UserId.String(), msg.Text})
							if err != nil {
								return finalSend(c, msg, chatState, "", err)
							}

							return nextStageTicketButton(c, msg, chatState, tBtn, ticket.GetService())
						}
					}
					// если не найдено значение то ошибка
					err = msg.Send(c, "Получено некорректное значение. Повторите попытку", nil)
					return database.CREATE_TICKET, err
				}

			case ticket.GetService():
				// получаем данные для заявок
				kinds, err := msg.GetTicketDataKinds(c, nil)
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}

				// если кнопка перехода к следующему шагу
				if btn != nil && btn.Goto == database.CREATE_TICKET {
					err = msg.Send(c, "Данный этап нельзя пропустить", nil)
					return database.CREATE_TICKET, err
				} else {
					for _, v := range kinds {
						if msg.Text == v.Name {
							err = msg.ChangeCacheTicket(c, chatState, varName, []string{v.ID.String(), msg.Text})
							if err != nil {
								return finalSend(c, msg, chatState, "", err)
							}

							return nextStageTicketButton(c, msg, chatState, tBtn, ticket.GetServiceType())
						}
					}

					// если не найдено значение то ошибка
					err = msg.Send(c, "Получено некорректное значение. Повторите попытку", nil)
					return database.CREATE_TICKET, err
				}

			case ticket.GetServiceType():
				types, err := msg.GetTicketDataTypesWhereKind(c, nil, state.Ticket.Service.Id)
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}

				// если кнопка перехода к следующему шагу
				if btn != nil && btn.Goto == database.CREATE_TICKET {
					err = msg.Send(c, "Данный этап нельзя пропустить", nil)
					return database.CREATE_TICKET, err
				} else {
					for _, v := range types {
						if msg.Text == v.Name {
							err = msg.ChangeCacheTicket(c, chatState, varName, []string{v.ID.String(), msg.Text})
							if err != nil {
								return finalSend(c, msg, chatState, "", err)
							}

							return nextStageTicketButton(c, msg, chatState, tBtn, "FINAL")
						}
					}

					// если не найдено значение то ошибка
					err = msg.Send(c, "Получено некорректное значение. Повторите попытку", nil)
					return database.CREATE_TICKET, err
				}

			// этап регистрации заявки
			case "FINAL":
				if btn != nil && btn.Goto == database.CREATE_TICKET {
					// удаляем клавиатуру
					err = msg.DropKeyboard(c)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					err = msg.Send(c, "Заявка регистрируется, ожидайте...", nil)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					// регистрируем заявку
					r, err := msg.ServiceRequestAdd(c, state.Ticket)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					// даем время чтобы загрузилась заявка
					for i := 0; i < 10; i++ {
						time.Sleep(4 * time.Second)

						_, err := msg.GetTicket(c, uuid.MustParse(r["ServiceRequestID"]))
						if err == nil {
							break
						}
					}

					// чистим данные
					err = msg.ClearCacheOmitemptyFields(c, chatState)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					return SendAnswer(c, msg, chatState, menu, tBtn.Goto, cnf.FilesDir, err)
				}
			}

			err = msg.Send(c, "Ожидалось нажатие на кнопку. Повторите попытку", nil)
			return database.CREATE_TICKET, err

		// пользователь попадет сюда в случае перехода в режим ожидания сообщения
		case database.WAIT_SEND:
			state := msg.GetState(c)

			// записываем введенные данные в переменную
			varName, ok := msg.GetCacheVar(c, database.VAR_FOR_SAVE)
			if ok && varName != "" {
				msg.ChangeCacheVars(c, chatState, varName, msg.Text)
			}

			// переходим если нажата BackButton
			btn := menu.GetButton(state.CurrentState, text)
			goTo := getGoToIfClickedBackBtn(btn, state)
			if goTo != "" {
				// чистим данные
				err = msg.ClearCacheOmitemptyFields(c, chatState)
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}

				return SendAnswer(c, msg, chatState, menu, goTo, cnf.FilesDir, err)
			}

			// выполнить действие кнопки
			err = msg.ChangeCacheState(c, chatState, database.START)
			if err != nil {
				return finalSend(c, msg, chatState, "", err)
			}
			return processMessage(c, msg, chatState)

		default:
			state := msg.GetState(c)
			currentMenu := state.CurrentState

			// В редисе может остаться состояние которого, нет в конфиге.
			cm, ok := menu.Menu[currentMenu]
			if !ok {
				logger.Warning("неизвестное состояние: ", currentMenu)
				err = msg.Send(c, menu.ErrorMessage, menu.GenKeyboard(database.START))
				return database.GREETINGS, err
			}

			// определяем какая кнопка была нажата
			btn := menu.GetButton(currentMenu, text)
			if btn == nil {
				text = strings.ReplaceAll(text, "«", "\"")
				text = strings.ReplaceAll(text, "»", "\"")
				btn = menu.GetButton(currentMenu, text)
			}

			// если есть принудительное значение для кнопки то присвоить
			if state.SavedButton != nil {
				btn = state.SavedButton

				// очищаем данные чтобы не было повторного использования
				err = msg.ClearCacheOmitemptyFields(c, chatState)
				if err != nil {
					return finalSend(c, msg, chatState, "", err)
				}
			}

			if btn != nil {
				goTo := btn.Goto
				if btn.BackButton {
					if state.PreviousState != database.GREETINGS {
						goTo = state.PreviousState
					} else {
						goTo = database.START
					}
				}

				for i := 0; i < len(btn.Chat); i++ {
					if btn.Chat[i].Chat != "" {
						r, err := fillTemplateWithInfo(c, msg, btn.Chat[i].Chat)
						if err != nil {
							return finalSend(c, msg, chatState, "", err)
						}

						msg.Send(c, r, nil)
					}
					if btn.Chat[i].File != "" {
						if isImage, filepath, err := getFileInfo(btn.Chat[i].File, cnf.FilesDir); err == nil {
							err := msg.SendFile(c, isImage, btn.Chat[i].File, filepath, &btn.Chat[i].FileText, nil)
							if err != nil {
								logger.Warning(err)
							}
						}
					}
					time.Sleep(250 * time.Millisecond)
				}
				if btn.CloseButton {
					// чистим данные
					err = msg.ClearCacheOmitemptyFields(c, chatState)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					err = msg.CloseTreatment(c)
					return database.GREETINGS, err
				}
				if btn.RedirectButton {
					err = msg.RerouteTreatment(c)
					return database.GREETINGS, err
				}
				if btn.AppointSpecButton != nil && *btn.AppointSpecButton != uuid.Nil {
					ok, err := msg.GetSpecialistAvailable(c, *btn.AppointSpecButton)
					if err != nil || !ok {
						return finalSend(c, msg, chatState, "Выбранный специалист недоступен", err)
					}
					err = msg.AppointSpec(c, *btn.AppointSpecButton)
					return database.GREETINGS, err
				}
				if btn.AppointRandomSpecFromListButton != nil && len(*btn.AppointRandomSpecFromListButton) != 0 {
					// получаем список свободных специалистов
					r, err := msg.GetSpecialistsAvailable(c)
					if err != nil || len(r) == 0 {
						return finalSend(c, msg, chatState, "Специалисты данной области недоступны", err)
					}

					// создаем словарь id специалистов которых мы хотели бы назначить
					specIDs := make(map[uuid.UUID]struct{})
					for _, id := range *btn.AppointRandomSpecFromListButton {
						specIDs[id] = struct{}{}
					}

					// ищем среди свободных специалистов нужных
					neededSpec := make([]uuid.UUID, 0)
					for i := 0; i < len(r); i++ {
						if _, exists := specIDs[r[i]]; exists {
							neededSpec = append(neededSpec, r[i])
						}
					}

					// проверяем есть ли хотя бы 1 свободный специалист
					lenNeededSpec := len(neededSpec)
					if lenNeededSpec == 0 {
						return finalSend(c, msg, chatState, "Специалисты данной области недоступны", err)
					}

					// назначаем случайного специалиста из списка
					seed := time.Now().UnixNano()
					rns := rand.NewSource(seed)
					rng := rand.New(rns)
					randomIndex := rng.Intn(lenNeededSpec)
					err = msg.AppointSpec(c, neededSpec[randomIndex])
					return database.GREETINGS, err
				}
				if btn.RerouteButton != nil && *btn.RerouteButton != uuid.Nil {
					r, err := msg.GetSubscriptions(c, *btn.RerouteButton)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}
					if len(r) == 0 {
						return finalSend(c, msg, chatState, "Выбранная линия недоступна", err)
					}

					err = msg.Reroute(c, *btn.RerouteButton, "")
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}
					return database.GREETINGS, err
				}
				if btn.ExecButton != "" {
					r, err := fillTemplateWithInfo(c, msg, btn.ExecButton)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					// выполняем команду на устройстве
					cmdParts := strings.Fields(r)
					var cmd = exec.Command(cmdParts[0], cmdParts[1:]...)
					cmdOutput, err := cmd.CombinedOutput()
					if err != nil {
						return finalSend(c, msg, chatState, "Ошибка: "+err.Error(), err)
					}

					// выводим результат и завершаем
					msg.Send(c, string(cmdOutput), nil)
					goTo := database.FINAL
					if btn.Goto != "" {
						goTo = btn.Goto
					}
					return SendAnswer(c, msg, chatState, menu, goTo, cnf.FilesDir, err)
				}
				if btn.SaveToVar != nil {
					// Сообщаем пользователю что требуем и запускаем ожидание данных
					if btn.SaveToVar.SendText != nil && *btn.SaveToVar.SendText != "" {
						r, err := fillTemplateWithInfo(c, msg, *btn.SaveToVar.SendText)
						if err != nil {
							return finalSend(c, msg, chatState, "", err)
						}

						msg.Send(c, r, menu.GenKeyboard(database.WAIT_SEND))
					} else {
						// выводим default WAIT_SEND меню в случае отсутствия настроек текста
						_, err := SendAnswer(c, msg, chatState, menu, database.WAIT_SEND, cnf.FilesDir, err)
						if err != nil {
							return finalSend(c, msg, chatState, "", err)
						}
					}

					// сохраняем имя переменной куда будем записывать результат
					err := msg.ChangeCacheVars(c, chatState, database.VAR_FOR_SAVE, btn.SaveToVar.VarName)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					// сохраняем ссылку на кнопку которая будет выполнена после завершения
					if btn.SaveToVar.DoButton != nil {
						err = msg.ChangeCacheSavedButton(c, chatState, btn.SaveToVar.DoButton)
						if err != nil {
							return finalSend(c, msg, chatState, "", err)
						}
					}

					return database.WAIT_SEND, err
				}
				if btn.TicketButton != nil {
					// сохраняем ссылку на кнопку которая была нажата
					err = msg.ChangeCacheSavedButton(c, chatState, btn)
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					t := database.Ticket{}

					// сохраняем id канала поступления заявки
					err = msg.ChangeCacheTicket(c, chatState, t.GetChannel(), []string{btn.TicketButton.ChannelID.String()})
					if err != nil {
						return finalSend(c, msg, chatState, "", err)
					}

					gt, err := nextStageTicketButton(c, msg, chatState, btn.TicketButton, t.GetTheme())
					return gt, err
				}

				// Сообщения при переходе на новое меню.
				return SendAnswer(c, msg, chatState, menu, goTo, cnf.FilesDir, err)

			} else { // Произвольный текст
				if !cm.QnaDisable && menu.UseQNA.Enabled {
					// logger.Info("QNA", msg, chatState)

					qnaText, isClose, request_id, result_id := getMessageFromQNA(msg, cnf)
					if qnaText != "" {
						// Была подсказка
						go msg.QnaSelected(cnf, request_id, result_id)

						if isClose {
							err = msg.Send(c, qnaText, nil)
							msg.CloseTreatment(c)
							return state.CurrentState, err
						}

						err = msg.Send(c, qnaText, menu.GenKeyboard(currentMenu))
						return state.CurrentState, err
					}

					return SendAnswer(c, msg, chatState, menu, database.FAIL_QNA, cnf.FilesDir, err)
				}
				err = msg.Send(c, menu.ErrorMessage, menu.GenKeyboard(currentMenu))
				return state.CurrentState, err
			}
		}
	case messages.MESSAGE_TREATMENT_TO_BOT,
		messages.MESSAGE_LINE_REROUTING_OTHER_LINE:
	default:
		panic(fmt.Sprintf("unexpected messages.MessageType: %#v", msg.MessageType))
	}
	return database.GREETINGS, errors.New("i don't know what i should do")
}

// выполнить Send и вывести Final меню
func finalSend(c *gin.Context, msg *messages.Message, chatState *messages.Chat, finalMsg string, err error) (string, error) {
	cnf := c.MustGet("cnf").(*config.Conf)
	menu := c.MustGet("menus").(*botconfig_parser.Levels)

	if finalMsg == "" {
		finalMsg = "Во время обработки вашего запроса произошла ошибка"
	}
	msg.Send(c, finalMsg, nil)

	// чистим данные чтобы избежать повторных ошибок
	msg.ClearCacheOmitemptyFields(c, chatState)

	return SendAnswer(c, msg, chatState, menu, database.FINAL, cnf.FilesDir, err)
}

// getMessageFromQNA - Метод возвращает ответ с Базы Знаний, и флаг, если это сообщение закрывает обращение.
func getMessageFromQNA(msg *messages.Message, cnf *config.Conf) (string, bool, uuid.UUID, uuid.UUID) {
	result_id := uuid.Nil
	qnaAnswer := msg.GetQNA(cnf, false, false)

	for _, v := range qnaAnswer.Answers {
		if v.Accuracy > 0 {
			result_id = v.ID
			return v.Text, v.AnswerSource == "GOODBYES", qnaAnswer.RequestID, result_id
		}
	}

	return "", false, result_id, result_id
}
