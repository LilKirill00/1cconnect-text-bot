package bot

import (
	"connect-text-bot/bot/messages"
	"connect-text-bot/bot/requests"
	"connect-text-bot/botconfig_parser"
	"connect-text-bot/config"
	"connect-text-bot/database"
	"connect-text-bot/logger"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/gin-gonic/gin"
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
		chatState := getState(c, &msg)

		newState, err := processMessage(c, &msg, &chatState)
		if err != nil {
			logger.Warning("Error processMessage", err)
		}

		err = changeState(c, &msg, &chatState, newState)
		if err != nil {
			logger.Warning("Error changeState", err)
		}
	}(cCp, msg)

	c.Status(http.StatusOK)
}

func changeState(c *gin.Context, msg *messages.Message, chatState *database.Chat, toState string) error {
	cache := c.MustGet("cache").(*bigcache.BigCache)

	if chatState.CurrentState == toState {
		return nil
	}

	chatState.PreviousState = chatState.CurrentState
	chatState.CurrentState = toState

	data, err := json.Marshal(chatState)
	if err != nil {
		logger.Warning("Error while change state to cache", err)
		return err
	}

	dbStateKey := msg.UserId.String() + ":" + msg.LineId.String()

	err = cache.Set(dbStateKey, data)
	logger.Debug("Write state to cache result")
	if err != nil {
		logger.Warning("Error while write state to cache", err)
	}

	return nil
}

func getState(c *gin.Context, msg *messages.Message) database.Chat {
	cache := c.MustGet("cache").(*bigcache.BigCache)

	var chatState database.Chat

	dbStateKey := msg.UserId.String() + ":" + msg.LineId.String()

	b, err := cache.Get(dbStateKey)
	if err != nil {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			logger.Info("No state in cache for " + msg.UserId.String() + ":" + msg.LineId.String())
			chatState = database.Chat{
				PreviousState: database.GREETINGS,
				CurrentState:  database.GREETINGS,
			}
			return chatState
		}
	}
	err = json.Unmarshal(b, &chatState)
	if err != nil {
		logger.Warning("Error while decoding state", err)
	}

	return chatState
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

func SendAnswer(c *gin.Context, msg *messages.Message, menu *botconfig_parser.Levels, goTo, filesDir string) {
	var toSend *[][]requests.KeyboardKey

	for i := 0; i < len(menu.Menu[goTo].Answer); i++ {
		// Отправляем клаву только с последним сообщением.
		// Т.к в дп4 криво отображается.
		if i == len(menu.Menu[goTo].Answer)-1 {
			toSend = menu.GenKeyboard(goTo)
		}
		if menu.Menu[goTo].Answer[i].Chat != "" {
			msg.Send(c, menu.Menu[goTo].Answer[i].Chat, toSend)
		}
		if menu.Menu[goTo].Answer[i].File != "" {
			if isImage, filePath, err := getFileInfo(menu.Menu[goTo].Answer[i].File, filesDir); err == nil {
				msg.SendFile(c, isImage, menu.Menu[goTo].Answer[i].File, filePath, &menu.Menu[goTo].Answer[i].FileText, toSend)
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func processMessage(c *gin.Context, msg *messages.Message, chatState *database.Chat) (string, error) {
	cnf := c.MustGet("cnf").(*config.Conf)
	menu := c.MustGet("menus").(*botconfig_parser.Levels)
	time.Sleep(250 * time.Millisecond)

	switch msg.MessageType {
	// Первый запуск.
	case messages.MESSAGE_TREATMENT_START_BY_USER:
		return database.GREETINGS, nil

	// Нажатие меню ИЛИ Любое сообщение (текст, файл, попытка звонка).
	case messages.MESSAGE_CALL_START_TREATMENT,
		messages.MESSAGE_CALL_START_NO_TREATMENT,
		messages.MESSAGE_TREATMENT_START_BY_SPEC,
		messages.MESSAGE_TREATMENT_CLOSE,
		messages.MESSAGE_TREATMENT_CLOSE_ACTIVE:
		err := msg.Start(cnf)
		return database.GREETINGS, err

	case messages.MESSAGE_NO_FREE_SPECIALISTS:
		err := msg.RerouteTreatment(c)
		return database.GREETINGS, err

	// Пользователь отправил сообщение.
	case messages.MESSAGE_TEXT,
		messages.MESSAGE_FILE:
		text := strings.ToLower(strings.TrimSpace(msg.Text))

		switch chatState.CurrentState {

		case database.GREETINGS:
			if menu.FirstGreeting {
				msg.Send(c, menu.GreetingMessage, nil)
			}
			SendAnswer(c, msg, menu, database.START, cnf.FilesDir)
			return database.START, nil
		default:
			var err error
			state := getState(c, msg)
			currentMenu := state.CurrentState

			// В редисе может остаться состояние которого, нет в конфиге.
			if _, ok := menu.Menu[currentMenu]; !ok {
				logger.Warning("неизвестное состояние: %s", currentMenu)
				err = msg.Send(c, menu.ErrorMessage, menu.GenKeyboard(database.START))
				return database.GREETINGS, err
			}

			btn := menu.GetButton(currentMenu, text)
			if btn == nil {
				text = strings.ReplaceAll(text, "«", "\"")
				text = strings.ReplaceAll(text, "»", "\"")
				btn = menu.GetButton(currentMenu, text)
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
					if btn.Chat[i].Chat != "" && !btn.CloseButton && !btn.RedirectButton {
						msg.Send(c, btn.Chat[i].Chat, nil)
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
					err = msg.CloseTreatment(c)
					return database.GREETINGS, err
				}
				if btn.RedirectButton {
					err = msg.RerouteTreatment(c)
					return database.GREETINGS, err
				}

				// Сообщения при переходе на новое меню.
				SendAnswer(c, msg, menu, goTo, cnf.FilesDir)
				return goTo, err

			} else { // Произвольный текст
				err = msg.Send(c, menu.ErrorMessage, menu.GenKeyboard(currentMenu))
				return state.CurrentState, err
			}
		}
	}
	return database.GREETINGS, errors.New("I don't know what i should do!")
}
