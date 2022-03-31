package messages

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"time"

	"connect-text-bot/bot/client"
	"connect-text-bot/bot/requests"
	"connect-text-bot/config"
	"connect-text-bot/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MessageType int

const (
	MESSAGE_TEXT                    MessageType = 1
	MESSAGE_CALL_START_TREATMENT    MessageType = 20
	MESSAGE_CALL_START_NO_TREATMENT MessageType = 21
	MESSAGE_FILE                    MessageType = 70
	MESSAGE_TREATMENT_START_BY_USER MessageType = 80
	MESSAGE_TREATMENT_START_BY_SPEC MessageType = 81
	MESSAGE_TREATMENT_CLOSE         MessageType = 82
	MESSAGE_NO_FREE_SPECIALISTS     MessageType = 83
	MESSAGE_TREATMENT_CLOSE_ACTIVE  MessageType = 90
	MESSAGE_TREATMENT_TO_BOT        MessageType = 200
)

type (
	Message struct {
		LineId uuid.UUID `json:"line_id" binding:"required" example:"4e48509f-6366-4897-9544-46f006e47074"`
		UserId uuid.UUID `json:"user_id" binding:"required" example:"4e48509f-6366-4897-9544-46f006e47074"`

		MessageID     uuid.UUID   `json:"message_id" binding:"required" example:"4e48509f-6366-4897-9544-46f006e47074"`
		MessageType   MessageType `json:"message_type" binding:"required" example:"1"`
		MessageAuthor *uuid.UUID  `json:"author_id" binding:"omitempty" example:"4e48509f-6366-4897-9544-46f006e47074"`
		MessageTime   string      `json:"message_time" binding:"required" example:"1"`
		Text          string      `json:"text" example:"Привет"`
		Data          struct {
			Redirect string `json:"redirect"`
		} `json:"data"`
	}

	AutofaqAnswer struct {
		Text         string  `json:"text"`
		Accuracy     float32 `json:"accuracy"`
		AnswerSource string  `json:"answer_source"`
	}

	AutofaqRequestBody struct {
		Question string          `json:"question"`
		Answers  []AutofaqAnswer `json:"answers"`
	}
)

// GetQNA - Метод позволяет получить варианты ответов на вопрос пользователя в сервисе AutoFAQ.
func (msg *Message) GetQNA(cnf *config.Conf) *AutofaqRequestBody {
	data := requests.DropKeyboardRequest{
		LineID: msg.LineId,
		UserId: msg.UserId,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Warning("text - GetQNA", err)
	}

	body, err := client.Invoke(cnf, http.MethodPost, "/line/qna/", "application/json", jsonData)
	if err != nil {
		logger.Warning("text - GetQNA", err)
	}

	resp := &AutofaqRequestBody{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		logger.Warning("text - GetQNA", err)
	}

	// Debug
	logger.Debug("text - GetQNA", resp)

	return resp
}

func (msg *Message) Start(cnf *config.Conf) error {
	data := requests.DropKeyboardRequest{
		LineID: msg.LineId,
		UserId: msg.UserId,
	}

	jsonData, err := json.Marshal(data)

	_, err = client.Invoke(cnf, "POST", "/line/drop/keyboard/", "application/json", jsonData)

	return err
}

func (msg *Message) Send(c *gin.Context, text string, keyboard *[][]requests.KeyboardKey) error {
	cnf := c.MustGet("cnf").(*config.Conf)

	data := requests.MessageRequest{
		LineID:   msg.LineId,
		UserId:   msg.UserId,
		AuthorID: cnf.SpecID,
		Text:     text,
		Keyboard: keyboard,
	}

	jsonData, err := json.Marshal(data)

	_, err = client.Invoke(cnf, "POST", "/line/send/message/", "application/json", jsonData)

	return err
}

func (msg *Message) RerouteTreatment(c *gin.Context) error {
	cnf := c.MustGet("cnf").(*config.Conf)

	data := requests.TreatmentRequest{
		LineID: msg.LineId,
		UserId: msg.UserId,
	}

	jsonData, err := json.Marshal(data)

	_, err = client.Invoke(cnf, "POST", "/line/appoint/start/", "application/json", jsonData)

	return err
}

func (msg *Message) CloseTreatment(c *gin.Context) error {
	cnf := c.MustGet("cnf").(*config.Conf)

	time.Sleep(500 * time.Millisecond)

	data := requests.TreatmentRequest{
		LineID: msg.LineId,
		UserId: msg.UserId,
	}

	jsonData, err := json.Marshal(data)

	_, err = client.Invoke(cnf, "POST", "/line/drop/treatment/", "application/json", jsonData)

	return err
}

func (msg *Message) StartAndReroute(cnf *config.Conf) error {
	msg.Start(cnf)
	data := requests.TreatmentRequest{
		LineID: msg.LineId,
		UserId: msg.UserId,
	}

	jsonData, err := json.Marshal(data)

	_, err = client.Invoke(cnf, "POST", "/line/appoint/start/", "application/json", jsonData)

	return err
}

func (msg *Message) SendFile(c *gin.Context, isImage bool, fileName string, filepath string, comment *string, keyboard *[][]requests.KeyboardKey) error {
	cnf := c.MustGet("cnf").(*config.Conf)

	data := requests.FileRequest{
		LineID:   msg.LineId,
		UserId:   msg.UserId,
		AuthorID: cnf.SpecID,
		FileName: fileName,
		Comment:  comment,
		Keyboard: keyboard,
	}

	jsonData, err := json.Marshal(data)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	metaPartHeader := textproto.MIMEHeader{}
	metaPartHeader.Set("Content-Disposition", `form-data; name="meta"`)
	metaPartHeader.Set("Content-Type", "application/json")
	metaPart, err := writer.CreatePart(metaPartHeader)
	if err != nil {
		return err
	}
	_, _ = metaPart.Write(jsonData)

	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	filePart, err := writer.CreateFormFile("file", fi.Name())
	if err != nil {
		return err
	}

	_, err = io.Copy(filePart, file)
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	if isImage {
		_, err = client.Invoke(cnf, "POST", "/line/send/image/", writer.FormDataContentType(), body.Bytes())
		return err
	}
	_, err = client.Invoke(cnf, "POST", "/line/send/file/", writer.FormDataContentType(), body.Bytes())
	return err
}
