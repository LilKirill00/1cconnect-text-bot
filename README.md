# Connect Text Bot

Данный бот реализует произвольное конфигурируемое текстовое меню на заданных линиях поддержки.

## Требования к окружению

* OS: Linux/Windows
* Go: 1.22+

## Сборка и запуск

### Сборка из исходников

```bash
./build.sh
```

**Note:** Сборка требует установленного окружения!

### Запуск собранной версии

```bash
./connect-text-bot --config=config.yml --bot=bot.yml
```

Где:

* `--config` - путь к конфигу (путь по умолчанию - `./config/config.yml`).
* `--bot` - путь к конфигу бота (путь по умолчанию - `./config/bot.yml`).

**Note:** Бот отслеживает изменения конфигурации меню, содержимое можно менять на горячую, но стоит предварительно
проверять через валидатор (например https://onlineyamltools.com/validate-yaml)

### Разворачивание бота

Для того чтобы бот работал корректно необходимо выполнить следующие требования и действия:

* Необходимо подготовить машину имеющую доступ в интернет и способную принимать HTTP запросы из интернета
  * Лучшим выбором будет **Linux**, возможно использование виртуальной машины
* Необходим полный тариф https://1c-connect.com/ru/forpartners/#2
* Настроить пользователя API в учетной системе 1С-Коннект
  * Раздел Администрирование -> Настройки API
  * Создаете нового пользователя
* Включить **Внешний сервер для обработки данных** в нужной линии
  * Откройте в УС карточку линии и в разделе **Чат-бот** включите соответствующую настройку
* Необходимо получить ID линии для которой была включена внешняя обработка
  * Выполнит запрос https://1c-connect.atlassian.net/wiki/spaces/PUBLIC/pages/2156429313/v1+line (можно открыть ссылку https://push.1c-connect.com/v1/line/ в браузере и ввести логин/пароль от ранее созданного пользователя)
  * Найти линию в списке и сохранить ее ID
* На подготовленный сервер загрузить приложение бота, файл с меню
* Сконфигурировать и запустить приложение
  * Создать конфигурационный файл. Пример лежит в файле `config/config.yml.sample` и отредактировать его
  * Указать в блоке **server** адрес к серверу на котором развернут бот
    * **Note:** Помните что указанный хост и порт должны быть доступны из сети Интернет
  * Указать логин/пароль ранее созданного пользователя API
  * Указать ID линии в разделе **lines**, можно указывать несколько линий
  * Бот может отправлять файлы, в конфигурационном файле можно указать путь к папке с файлами, далее в меню указывать имена файлов для отправки в чат
  * Приложение может быть запущено с указание путей к соответствующим файлам

## Конфигурация меню

Конфигурационный файл представляет собой `yml` файл вида:

```yaml
use_qna:
  enabled: true

menus:
  start:
    answer:
      - chat: 'Здравствуйте.'
    buttons:
      - button:
          id: 1
          text: 'a'
      - button:
          id: 2
          text: 'b'
      - button:
          id: 3
          text: 'nested'
          menu:
            id: 'nested_menu'
            answer:
              - chat: 'Welcome to nested menu.'
            buttons:
              - button:
                  id: 1
                  text: 'get information'
                  chat:
                    - chat: 'information'
              - button:
                  back_button: true
      - button:
          id: 4
          text: 'send file'
          chat:
            - file: 'file.pdf'
              file_text: 'you received file!'
  final_menu:
    answer:
      - chat: 'Могу ли я вам чем-то еще помочь?'
    buttons:
      - button:
          id: 1
          text: 'Да'
          goto: 'start'
      - button:
          id: 2
          text: 'Нет'
          chat:
            - chat: 'Спасибо за обращение!'
          close_button: true
      - button:
          redirect_button: true

back_button:
  id: 8
  text: 'Назад'

redirect_button:
  id: 0
  text: 'Соединить со специалистом'

close_button:
  id: 9
  text: 'Закрыть обращение'

error_message: 'Команда неизвестна. Попробуйте еще раз'
```

**Note:** Директория с файлами задается параметром `files_dir`, в конфигурационном файле программы `config.yml`.

Конфигурация состоит из различных меню. Меню `start` - появляется после первого сообщения от пользователя. `final_menu` - резюмирует диалог.

Каждое меню состоит из блоков `answer` и `buttons`.

Блок `answer` отвечает за сообщение при переходе на данный раздел.

При переходе между меню есть возможность отправить текст:

```yaml
menus:
  start:
    answer:
      - chat: 'Здравствуйте.'
    buttons:
    ...
```

Или файл:

```yaml
menus:
  start:
    answer:
      - file: 'file.pdf'
        file_text: 'Сопроводительное письмо к файлу.'
    buttons:
    ...
```

Или несколько сообщений и файлов:

```yaml
menus:
  start:
    answer:
      - chat: 'Сообщение 1'
      - chat: 'Сообщение 2'
      - file: 'file1.pdf'
      - file: 'file2.pdf'
      - chat: 'Сообщение 3'
    buttons:
    ...
```

Также при нажатии на кнопку есть возможность отправить несколько сообщений или файлов:

```yaml
menus:
  start:
    answer:
      - chat: 'Сообщение 1'
    buttons:
      - button:
          id: 1
          text: 'Кнопка 1'
          chat:
            - chat: 'Сообщение 1'
            - chat: 'Сообщение 2'
            - file: 'file1.pdf'
              file_text: 'Сопроводительное письмо к файлу1'
            - file: 'file2.pdf'
              file_text: 'Сопроводительное письмо к файлу2'
    ...
```

Блок `buttons` - представляет собой список кнопок на данном уровне. У кнопки обязательно должен текст `text`.

```yaml
buttons:
  - button:
      id: 1
      text: 'Текст кнопки' # обязательное поле
```

Вместо блока `buttons` можно использовать блок `do_button` что позволяет выполнить действие кнопки при попадание в это меню. В примере ниже представлено выполнение перевода на специалиста если не найден ответ из базы знаний

Блоки `buttons` и `do_button` нельзя использовать одновременно.

```yaml
menus:
...
  fail_qna_menu:
    answer:
      - chat: "Уважаемый клиент, перевожу Вас на специалиста Линии консультаций для решения данного вопроса!"
    do_button:
      redirect_button: true
```

Если у кнопки есть пункт `menu`, то после нажатия на неё будет совершен переход в подменю.

```yaml
buttons:
  - button:
      id: 1 # Нажатие на эту кнопку переведёт в nested_menu
      text: 'Текст кнопки'
      chat: 'Сообщение'
      menu:
        id: 'nested_menu'
        ...
```

### Как разбить конфигурационное меню на разные файлы

Конфигурационное меню позволяет разделять настройки на несколько файлов с помощью анкор (маркеров) `&` и ссылок на эти анкоры `*`. Это позволяет сделать ваш код более организованным и удобным для поддержки.

Файлы которые будут использоваться для поиска анкор должны находиться рядом с главным файлом конфигурации. Пример как можно расположить файлы:

```yaml
config/
├── main.yml  # Главный файл конфигурации
├── соседний_файл_конфигурации.yml
└── папка/
    ├── вложенный_файл_конфигурации.yml
    └── вложенная_папка/
        └── ...
```

Пример главного файла (main.yml)

```yaml
select_what_u_want: &select_what_u_want '{{ .User.Name }}, Выберите, что вас интересует :point_down::'

menus:
  start:
    answer:
      - chat: *select_what_u_want  # Ссылка на анкор с получением значений
    buttons:         
      - <<: *узнать_что_находится_по_координатам  # Ссылка на анкор со слиянием
      - button: *сказать_спасибо  # Ссылка на анкор с получением значений
```

Пример файла с анкором (любой другой файл с расширением `.yml`)

```yaml
&узнать_что_находится_по_координатам
  button: # lvl:1
    id: узнать_что_находится_по_координатам
    text: "Узнать что находится по координатам"
    save_to_var: 
      var_name: lat
      send_text: "Введите широту"
      do_button: # lvl:2
        save_to_var: 
          var_name: lon
          send_text: "Введите долготу" 
          do_button:  # lvl:3
            exec_button: './scripts/example.sh {{ .User.UserID }} {{ .Var.lat }} {{ .Var.lon }}'

&сказать_спасибо
  chat:
    - chat: "Уважаемый {{ .User.Name }}. Спасибо что нажали на эту кнопку"
```

### Настройки по умолчанию

Для специальных пунктов меню:

`back_button` - описывает кнопку "Назад", которая переводит меню на уровень назад.

`close_button` - описывает кнопку "Закрыть обращение", которая завершает работу с обращением.

`redirect_button` - описывает кнопку "Перевести на специалиста", которая переводит работу из бот-меню на свободного
специалиста или ставит обращение в очередь, если нет свободных специалистов.

Можно задать описания по умолчанию:

```yaml
back_button:
  id: 8
  text: 'Назад'

redirect_button:
  id: 0
  text: 'Соединить со специалистом'

close_button:
  id: 9
  text: 'Закрыть обращение'
```

Если в конфиге отсутствует `final_menu`, будет использовано меню по умолчанию:

```yaml
menus:
...
  final_menu:
    answer:
      - chat: 'Могу ли я вам чем-то еще помочь?'
    buttons:
      - button:
          id: 1
          text: 'Да'
          goto: 'start'
      - button:
          id: 2
          text: 'Нет'
          chat:
            - chat: 'Спасибо за обращение!'
          close_button: true
      - button:
          redirect_button: true
...
```

Можно сделать сделать так, чтобы бот здоровался только один раз.

Для этого необходимо добавить следующую строчку в конфиг бота (файл `bot.yml`):

```yaml
first_greeting: true
```

А также задать текст приветственного сообщения (файл `bot.yml`):
```yaml
greeting_message: 'Здравствуйте.'
```

Можно настроить текста ошибок, которые может получить пользователь, если какой-то параметр не настроен, то будет использовано для него значение по умолчанию также как в примере:

```yaml
error_messages:
  command_unknown: 'Команда неизвестна. Попробуйте еще раз'
  button_processing: 'Во время обработки вашего запроса произошла ошибка'
  failed_send_file: 'Ошибка: Не удалось отправить файл'
  appoint_spec_button:
    selected_spec_not_available: 'Выбранный специалист недоступен'
  appoint_random_spec_from_list_button:
    specs_not_available: 'Специалисты данной области недоступны'
  reroute_button:
    selected_line_not_available: 'Выбранная линия недоступна'
  ticket_button:
    step_cannot_be_skipped: 'Данный этап нельзя пропустить'
    received_incorrect_value: 'Получено некорректное значение. Повторите попытку'
    expected_button_press: 'Ожидалось нажатие на кнопку. Повторите попытку'
```

### Как отправить текст

```yaml
buttons:
  - button:
      id: 1
      text: 'Текст кнопки'
      chat:
        - chat: 'Сообщение'
```

### Как отправить файл

```yaml
buttons:
  - button:
      id: 1
      text: 'Текст кнопки'
      chat:
        - file: 'file.pdf'
          file_text: 'Сопроводительное сообщение к файлу.'
```

### Как закрыть обращение

```yaml
buttons:
  - button:
      id: 9
      text: 'Закрыть обращение'
      close_button: true
```

### Как перейти в определенное меню

Для перехода в определенное меню используется `goto`.

В качестве аргументов `goto` может принять:
- название созданного меню
- название специальных меню (`start`, `final_menu` или `fail_qna_menu`)
- id вложенного меню

```yaml
menus:
  start:
    answer:
      - chat: "Выберите куда хотите отправиться через goto"
    buttons:
      - button:
          text: "Перейти в goto_example"
          goto: "goto_example" # goto ссылается на меню goto_example
      - button:
          text: "Перейти в вложенное_меню"
          goto: "вложенное_меню" # goto ссылается на вложенное меню

  goto_example:
    answer:
      - chat: 'Вы попали в goto_example'
    buttons:
      - button:
          text: "Показать вложенное меню"
          menu:
            id: 'вложенное_меню'
            answer:
              - chat: "Вы во вложенном меню"
            buttons:
              - button:
                  text: "Вернуться в start"
                  goto: "start" # goto ссылается на меню start
              - button:
                  text: "Перейти в final_menu"
                  # goto не указан, но поскольку продолжения у кнопки нет то выполняется переход в final_menu
```

Если у кнопки нет продолжения и не настроен `goto`, то по умолчанию выполнится переход в `final_menu`

### Как вернуться в предыдущее меню

```yaml
buttons:
  - button:
      text: 'Назад'
      back_button: true
```

По нажатию на `back_button` пользователя вернет в предыдущее меню в котором он находился. Более понятно через пример:

|На какую кнопку пользователь нажал|Куда попал|На какое меню теперь ссылается back_button|
|----------|----------|----------|
|Меню|start|start|
|2.1|lvl_2.1|start|
|3.1|lvl_3.1|lvl_2.1|
|Перейти в goto_example|goto_example|lvl_3.1|
|Назад|lvl_3.1|lvl_2.1|
|Назад|lvl_2.1|start|
|Назад|start|start|

```yaml
menus:
  start:
    answer:
      - chat: "Выберите пункт меню"
    buttons: # lvl:1
      - button: 
          text: '2.1'
          menu:
            id: 'lvl_2.1'
            answer:
              - chat: "Вы на уровне 2.1"
            buttons: # lvl:2
              - button:
                  text: "Назад"
                  back_button: true 
              - button:
                  text: '3.1'
                  menu:
                    id: 'lvl_3.1'
                    answer:
                      - chat: "Вы на уровне 3.1"
                    buttons: # lvl:3  
                      - button:
                          text: "Назад"
                          back_button: true
                      - button:
                          text: "Перейти в goto_example"
                          goto: "goto_example"

  goto_example:
    answer:
      - chat: 'Вы попали в goto_example'
    buttons:
      - button:
          text: "Назад"
          back_button: true
```

### Как перевести на свободного специалиста

```yaml
buttons:
  - button:
      id: 0
      text: 'Перевести на свободного специалиста'
      redirect_button: true
```

### Как перевести на конкретного специалиста

```yaml
buttons:
  - button:
      id: 2
      text: 'Соединить со специалистом Иванов И.И.'
      appoint_spec_button: bb296731-3d58-4c4a-8227-315bdc2bf3ff
```

### Как перевести на случайного специалиста из списка

```yaml
buttons:
  - button:
      id: 2
      text: 'Соединить с одним из консультантов'
      appoint_random_spec_from_list_button:
            - bb296731-3d58-4c4a-8227-315bdc2bf1ff
            - bb296731-3d58-4c4a-8227-315bdc2bf2ff
            - bb296731-3d58-4c4a-8227-315bdc2bf3ff
```

### Как перевести обращение на другую линию

```yaml
buttons:
  - button:
      id: 3
      text: 'Перевод обращения на линию "1С-Коннект: Техподдержка"'
      reroute_button: bb296731-3d58-4c4a-8227-315bdc2bf3ff
```

### Как выполнить команду на стороне сервера

```yaml
buttons:
  - button:
      id: 3
      text: 'Выполнить команду на стороне сервера'
      exec_button: "./scripts/example.sh {{ .User.UserID }} Имя: {{ .User.Name }}"
```

В команду можно передать:
- Статические данные (например, команды и аргументы).
- Динамические данные, которые можно получить с помощью шаблонов. Дополнительную информацию о шаблонах можно найти в разделе [Как пользоваться шаблонами](#как-пользоваться-шаблонами)

Скрипт `example.sh` имеет следующее содержание

```bash
#!/bin/bash

echo -n $1 | base64 
echo -n $2 $3
```

Не забудьте сделать скрипт исполняемым

```bash
chmod +x ./scripts/example.sh
```

### Как пользоваться шаблонами

#### Важные замечания:
- Шаблоны можно использовать в сообщениях, отправляемых от лица бота.
- Шаблоны можно использовать только в некоторых командах.
- Использовать шаблоны в тексте кнопок нельзя.

#### Как использовать шаблоны:
Чтобы вставить данные в текст, используйте синтаксис `{{ .ВидДанных.НазваниеПоля }}`. Просто вставьте его в нужное место вашего текста.

#### Доступные виды данных:
- `{{ .User.НазваниеПоля }}`: данные, относящиеся к объекту [User (Пользователь)](https://1c-connect.atlassian.net/wiki/spaces/PUBLIC/pages/1289355329/User)
- `{{ .Var.НазваниеПеременной }}`: данные, полученные от сообщения, отправленного пользователем. Подробнее в [Как получить и сохранить текст введенный пользователем](#как-получить-и-сохранить-текст-введенный-пользователем)

#### Пример использования:

```yaml
menus:  
  start:
    answer:
      - chat: '{{ .User.Name }}, Выберите, что вас интересует :point_down::'
    buttons:
      - button:
          id: 3
          text: 'Выполнить команду на стороне сервера'
          chat:
            - chat: "Уважаемый {{ .User.Name }}. Сейчас происходит обработка на стороне сервера, подождите немного"
          exec_button: "./scripts/example.sh {{ .User.UserID }} {{ .User.Surname }} {{ .User.Name }}"
```

### Как получить и сохранить текст введенный пользователем

```yaml
menus:  
  start:
    answer:
      - chat: *select_what_u_want
    buttons:          
      - button: # lvl:1
          id: 100
          text: "Узнать что находится по координатам"
          save_to_var: 
            var_name: lat
            send_text: "Введите широту или выберите один из предложенных вариантов"
            offer_options:
              - 37.7749° N
              - 148.8566° N
              - 255.7558° N
              - -33.4489° S
              - 35.6895° N
            do_button: # lvl:2
              save_to_var: 
                var_name: lon
                send_text: "Введите долготу" 
                do_button: # lvl:3
                  goto: coords_check   

  coords_check:
    answer:
      - chat: 'Подтвердите операцию'
    buttons:
      - button:
          text: "Назад в меню"
          goto: start
      - button:
          text: "Подтвердить"
          exec_button: './scripts/example.sh {{ .User.UserID }} {{ .Var.lat }} {{ .Var.lon }}'
```

Параметры `save_to_var`:
- `var_name`: имя переменной, в которую будет сохранен результат. Это позволяет использовать значение позже в [шаблонах](#как-пользоваться-шаблонами).
- `send_text`: сообщение, которое увидит пользователь после нажатия на кнопку. Если этот параметр оставить пустым, пользователю отправится сообщение по умолчанию.
- `offer_options`: список значений из которых пользователь может выбрать ответ.
- `do_button`: действие которое выполнится после получения сообщения от пользователя. Сработает также как при нажатие пользователем кнопки (например, [выполнить команду на стороне сервера](#как-выполнить-команду-на-стороне-сервера)) .

Если в конфиге отсутствует `wait_send_menu`, будет использовано меню по умолчанию. Данное меню позволит настроить сообщение которое видит пользователь если не указать параметр send_text, а также тут можно настроить какие кнопки будет видеть пользователь на всех кнопках save_to_var:

```yaml
menus:
...
  wait_send_menu:
    answer:
      - chat: "Текст запроса сообщения если не указан параметр send_text"
    buttons:
      - button:
          id: 1
          text: 'Текст кнопки для отмены действий'
          back_button: true
```

### Как зарегистрировать заявку

```yaml
menus:  
  start:
    answer:
      - chat: *select_what_u_want
    buttons:       
      - button:
          text: "Зарегистрировать заявку"
          ticket_button:
              channel_id: bb296731-3d58-4c4a-8227-315bdc2bf3ff
              ticket_info: |
                Ваша новая заявка:
                Тема: {{ .Ticket.Theme }}
                Описание {{ .Ticket.Description }}
                Исполнитель: {{ .Ticket.Executor.Name }}
                Услуга: {{ .Ticket.Service.Name }}
                Вид работ: {{ .Ticket.ServiceType.Name }}
              goto: start
              data: # Данные заявки
                theme: # Тема
                  text: "Введите тему или нажмите «Далее»" 
                  value: "Возникла проблема"
                description: # Описание
                  text: "{{ .User.Name }} Введите описание или нажмите «Далее»"
                  value: "Тут видите пример вставки шаблона: {{ .User.Name }}"
                executor: # Исполнитель
                  text: "Выберите исполнителя"
                  value: bb296731-3d58-4c4a-8227-315bdc2bf3ff
                service: # Услуга
                  text: "Выберите услугу"
                  value: bb296731-3d58-4c4a-8227-315bdc2bf3ff
                type: # Тип услуги
                  text: "Выберите вид работ"
                  value: bb296731-3d58-4c4a-8227-315bdc2bf3ff
```

Пройдемся по некоторым параметрам:
- `channel_id` - id канала откуда поступает заявка
- `ticket_info` - шаблон информации о заявке, который будет отображаться на каждом шаге формирования заявки
- `goto` - необязательный параметр. перейти в определенное меню при нажатие на отмену или завершение заявки
- `text` - необязательный параметр, если указано `value`. текст который определяет какое сообщение будет на шаге
- `value` - необязательный параметр, если указан `text`. значение по умолчанию, которое если указано, то будет пропущен шаг

Примечания:
- необходимо настроить каждый шаг для того чтобы кнопка работала.
- для каждого шага необходимо указать `text` или `value`, если указать оба параметра, то использоваться будет только значение `value`.
- параметры `value` для `executor, service, type` должны быть id.
- не рекомендуется указывать `value` для `type` если не указано `value` для `service`.

### Как создать меню

#### Способ №1

```yaml
menus:
  start:
    answer:
      - chat: 'Здравствуйте.'
    buttons:
      - button:
          id: 1
          text: 'a'
          menu:
            id: 'новое_меню'
            answer:
              - chat: 'welcome'
            buttons:
              - button:
                  id: 1
                  text: 'Текст кнопки'
  final_menu:
    answer:
      - chat: 'Могу ли я вам чем-то еще помочь?'
    buttons:
      - button:
          id: 1
          text: 'Да'
          goto: 'start'
      - button:
          id: 2
          text: 'Нет'
          chat:
            - chat: 'Спасибо за обращение!'
          close_button: true
      - button:
          redirect_button: true
back_button:
  id: 8
  text: 'Назад'

redirect_button:
  id: 0
  text: 'Соединить со специалистом'

close_button:
  id: 9
  text: 'Закрыть обращение'
```

#### Способ №2

```yaml
menus:
  start:
    answer:
      - chat: 'Здравствуйте.'
    buttons:
      - button:
          id: 1
          text: 'a'
          goto: 'новое_меню'

  новое_меню:
    answer:
      - chat: 'welcome'
    buttons:
      - button:
          id: 1
          text: 'Текст кнопки'

  final_menu:
    answer:
      - chat: 'Могу ли я вам чем-то еще помочь?'
    buttons:
      - button:
          id: 1
          text: 'Да'
          goto: 'start'
      - button:
          id: 2
          text: 'Нет'
          chat:
            - chat: 'Спасибо за обращение!'
          close_button: true
      - button:
          redirect_button: true
back_button:
  id: 8
  text: 'Назад'

redirect_button:
  id: 0
  text: 'Соединить со специалистом'

close_button:
  id: 9
  text: 'Закрыть обращение'
```

## Использование подсказок из баз знаний

### Глобальные параметры использования база знаний компании задаются разделом `use_qna`

```yaml
use_qna:
  enabled: true
```

`enabled` - включен поиск ответов в базах знаний

### В конкретном меню можно отключить использование подсказок, воспользовавшись параметром `qna_disable`

```yaml
...
      menu:
        ...
        qna_disable: true
...
```

Если в конфиге отсутствует `fail_qna_menu`, будет использовано меню по умолчанию в случае отсутствия ответа на произвольный вопрос:

```yaml
menus:
...
  fail_qna_menu:
    answer:
      - chat: |
          Я Вас не понимаю.

          Попробуете еще раз или перевести обращение на специалиста?
    buttons:
      - button:
          id: 1
          text: 'Продолжить'
          back_button: true
      - button:
          id: 2
          text: 'Закрыть обращение'
          chat:
            - chat: 'Спасибо за обращение!'
          close_button: true
      - button:
          redirect_button: true
...
```
