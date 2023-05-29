package botconfig_parser

type Levels struct {
	Menu map[string]*Menu `yaml:"menus"`

	UseQNA QNA `yaml:"use_qna"`

	BackButton     *Button `yaml:"back_button"`
	CloseButton    *Button `yaml:"close_button"`
	RedirectButton *Button `yaml:"redirect_button"`

	ErrorMessage    string `yaml:"error_message"`
	GreetingMessage string `yaml:"greeting_message"`
	FirstGreeting   bool   `yaml:"first_greeting"`
}

type Menu struct {
	Answer  []*Answer  `yaml:"answer"`
	Buttons []*Buttons `yaml:"buttons"`

	QnaDisable bool `yaml:"qna_disable"`
}

type QNA struct {
	Enabled bool `yaml:"enabled"`
}

type Answer struct {
	Chat     string `yaml:"chat"`                // сообщние при переходе на меню
	File     string `yaml:"file,omitempty"`      // путь к файлу
	FileText string `yaml:"file_text,omitempty"` // сопроводительный текст к файлу
}

type Buttons struct {
	Button Button
}

type NestedMenu struct {
	ID      string     `yaml:"id"`
	Answer  []*Answer  `yaml:"answer"`
	Buttons []*Buttons `yaml:"buttons"`

	QnaDisable bool `yaml:"qna_disable"`
}

type Button struct {
	ButtonID       string      `yaml:"id"`                        // id кнопки
	ButtonText     string      `yaml:"text"`                      // текст кнопки
	Chat           []*Answer   `yaml:"chat,omitempty"`            // сообщение
	CloseButton    bool        `yaml:"close_button,omitempty"`    // закрыть обращение
	RedirectButton bool        `yaml:"redirect_button,omitempty"` // перевести на специалиста
	BackButton     bool        `yaml:"back_button,omitempty"`     // вернуться назад
	Goto           string      `yaml:"goto"`                      // перейти в меню
	NestedMenu     *NestedMenu `yaml:"menu"`                      // вложенное меню
}

func (b *Button) SetDefault(default_ Button) {
	if b.ButtonID == "" {
		b.ButtonID = default_.ButtonID
	}
	if b.ButtonText == "" {
		b.ButtonText = default_.ButtonText
	}
	if len(b.Chat) == 0 {
		b.Chat = default_.Chat
	}
}
