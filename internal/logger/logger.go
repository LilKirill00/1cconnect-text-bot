package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/goccy/go-yaml"
)

var (
	isDebug = false

	CritColor    = color.RGB(255, 0, 0).SprintFunc()
	DebugColor   = color.RGB(255, 165, 0).SprintFunc()
	WarningColor = color.RGB(255, 255, 0).SprintFunc()
	EventColor   = color.RGB(0, 255, 0).SprintFunc()
)

type (
	loggerConfig struct {
		Logging *struct {
			// Сохранять ли логи
			Enabled bool `yaml:"enabled"`
			// В какую папку сохранять по умолчанию "./log"
			Directory string `yaml:"directory"`
			// Формат даты и времени в имени файла
			FilenameFormat string `yaml:"filename_format"`
		} `yaml:"logging"`

		// Настройки цветов
		Color *struct {
			// Отключить все цвета
			NoColor bool `yaml:"no_color"`

			Crit    colorConf `yaml:"crit"`
			Debug   colorConf `yaml:"debug"`
			Warning colorConf `yaml:"warning"`
			Event   colorConf `yaml:"event"`
		} `yaml:"color"`
	}

	colorConf struct {
		Enabled bool    `yaml:"enabled"`
		Rgb     *[3]int `yaml:"rgb"`
	}
)

func InitLogger(debug bool, configPath *string) *os.File {
	isDebug = debug
	color.NoColor = true

	log.SetPrefix("[APP] ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)

	// читаем настройки
	input, err := os.Open(*configPath)
	if err != nil {
		Info("Настройки для логов не найдены")
		return nil
	}
	defer input.Close()

	decoder := yaml.NewDecoder(input)
	cnf := &loggerConfig{}
	err = decoder.Decode(cnf)
	if err != nil {
		Warning("Ошибка загрузки настроек для логов", err)
		return nil
	}

	// настраиваем цвета
	if cnf.Color != nil && !cnf.Color.NoColor {
		color.NoColor = cnf.Color.NoColor

		setColorCnf := func(cData colorConf, globColor *func(a ...interface{}) string) {
			if !cData.Enabled {
				d := new(color.Color)
				d.DisableColor()
				*globColor = d.SprintFunc()
				return
			}
			if cData.Rgb != nil {
				*globColor = color.RGB((*cData.Rgb)[0], (*cData.Rgb)[1], (*cData.Rgb)[2]).SprintFunc()
			}
		}

		setColorCnf(cnf.Color.Crit, &CritColor)
		setColorCnf(cnf.Color.Debug, &DebugColor)
		setColorCnf(cnf.Color.Warning, &WarningColor)
		setColorCnf(cnf.Color.Event, &EventColor)
	}

	// настраиваем сохранение
	if cnf.Logging != nil && cnf.Logging.Enabled {
		if cnf.Logging.Directory == "" {
			cnf.Logging.Directory = "./log"
		}

		if cnf.Logging.FilenameFormat == "" {
			cnf.Logging.FilenameFormat = "app"
		}

		fileName := fmt.Sprintf("%s/%s.log", cnf.Logging.Directory, time.Now().Format(cnf.Logging.FilenameFormat))

		logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			Warning("Ошибка связанная с файлом записи логов, в данный момент логи не сохраняются: ", err)
			return nil
		}
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)

		return logFile
	}

	return nil
}

func Info(v ...interface{}) {
	log.Print("[INFO] ", fmt.Sprintln(v...))
}

func Event(v ...interface{}) {
	log.Print(EventColor("[Event] ", fmt.Sprintln(v...)))
}

func Warning(v ...interface{}) {
	log.Print(WarningColor("[WARNING] ", fmt.Sprintln(v...)))
}

func Debug(v ...interface{}) {
	if isDebug {
		message := new(bytes.Buffer)

		for _, str := range v {
			v, ok := str.(string)
			if ok {
				_, _ = fmt.Fprintf(message, "%s ", v)
			} else {
				s, _ := json.MarshalIndent(str, "", " ")
				_, _ = fmt.Fprintf(message, "%s ", string(s))
			}
		}

		log.Print(DebugColor("[DEBUG] ", message))
	}
}

func Crit(v ...interface{}) {
	log.Printf(CritColor("Critical error: %s"), v)
	time.Sleep(5 * time.Second)
	os.Exit(1)
}
