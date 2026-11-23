# Answerd

Быстрый ответ на тестовые вопросы с вариантами ответа в KDE Plasma: копируете вопрос (или делаете скриншот) — демон спрашивает DeepSeek — на панели вместо года в дате показывается номер правильного варианта, пока зажата **Ctrl**.

Проект — **Answerd**, виджет на панели — **Answerd Clock**.

```
12:34
11/20/26    ← обычный вид
11/20/01    ← Ctrl зажат: правильный вариант 1
11/20/13    ← Ctrl зажат: правильные варианты 1 и 3
11/20/134   ← Ctrl зажат: правильные варианты 1, 3 и 4
11/20/00    ← Ctrl зажат: ответ ещё обрабатывается
```

- `daemon/` — Go-демон: текст из буфера или скриншот → DeepSeek → `~/.cache/answerd/latest.json`.
- `widget/` — плазмоид **Answerd Clock** (форк KDE Digital Clock из `plasma-workspace`, GPL-2.0-or-later): читает `latest.json` и подменяет год.

## Требования

- KDE Plasma 6, сессия Wayland.
- `wl-clipboard` (команда `wl-paste`) и `spectacle`.
- Go ≥ 1.22 — только для сборки.
- API-ключ DeepSeek.

```sh
# Arch / Manjaro
sudo pacman -S wl-clipboard spectacle go

# Debian / Ubuntu
sudo apt install wl-clipboard spectacle golang-go
```

## Установка

### Быстрая установка

```sh
./answerd.sh --install
```

Скрипт собирает демон в `~/.local/bin/answerd`, создаёт `~/.config/answerd/config.json` и спрашивает API-ключ (если файла ещё нет), ставит KDE-автозапуск, копирует виджет с подстановкой ваших путей и запускает демон.

| Флаг | Действие |
|---|---|
| `--install` | установить (обязателен) |
| `--autostart` | добавить в автозапуск (по умолчанию для `--install`) |
| `--no-autostart` | не добавлять в автозапуск |
| `--no-start` | не запускать демон |
| `--restart-plasma` | перезапустить панель после установки |
| `--uninstall` | удалить демон, виджет и автозапуск |
| `--purge` | с `--uninstall`: удалить ещё конфиг и кэш |

Примеры:

```sh
./answerd.sh --install --no-autostart
./answerd.sh --uninstall
./answerd.sh --uninstall --purge
```

После установки перезапустите Plasma, добавьте **Answerd Clock** на панель и включите **Show date** и **Date format → Custom** → `MM/dd/yy`. Рекомендуется включить отображение секунд: **Show seconds → Always**.

<details>
<summary>Установка вручную</summary>

#### 1. Собрать демон

```sh
mkdir -p ~/.local/bin
cd daemon
go build -trimpath -o ~/.local/bin/answerd .
cd ..
```

`~/.local/bin` должен быть в `PATH`.

#### 2. Ключ API

```sh
mkdir -p ~/.config/answerd
cp daemon/config.example.json ~/.config/answerd/config.json
chmod 600 ~/.config/answerd/config.json
$EDITOR ~/.config/answerd/config.json
```

Впишите `api_key`. Вместо файла можно задать `DEEPSEEK_API_KEY` — переменная приоритетнее.

#### 3. Автозапуск

KDE-автозапуск (рекомендуется):

```sh
mkdir -p ~/.config/autostart
cp daemon/packaging/answerd.desktop ~/.config/autostart/
```

Запустить прямо сейчас:

```sh
~/.local/bin/answerd run &
```

Вариант systemd:

```sh
mkdir -p ~/.config/systemd/user
cp daemon/packaging/answerd.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now answerd
```

> Если сервис не видит Wayland-дисплей, используйте KDE-автозапуск: он наследует окружение сессии.

#### 4. Виджет

```sh
rm -rf ~/.local/share/plasma/plasmoids/answerd
cp -r widget ~/.local/share/plasma/plasmoids/answerd
```

В `DigitalClock.qml` два плейсхолдера — замените их на свои пути:

```sh
qml=~/.local/share/plasma/plasmoids/answerd/contents/ui/DigitalClock.qml
sed -i "s|@ANSWERD_BIN@|$HOME/.local/bin/answerd|g" "$qml"
sed -i "s|@ANSWERD_LATEST@|$HOME/.cache/answerd/latest.json|g" "$qml"
```

Проверка — плейсхолдеров остаться не должно, а пути должны быть абсолютными:

```sh
grep -n '@ANSWERD_' "$qml" || echo "placeholders replaced"
grep -n 'answerd screenshot\|latest.json' "$qml"
```

Перезапустите Plasma и добавьте виджет:

```sh
systemctl --user restart plasma-plasmashell.service
```

Если юнита нет — `plasmashell --replace >/dev/null 2>&1 &`.

Правый клик по панели → **Enter Edit Mode** → **Add Widgets** → **Answerd Clock**.

В настройках виджета включите **Show date**, выберите **Date format → Custom** и укажите `MM/dd/yy`.

#### 5. Проверка

```sh
pgrep -af answerd
tail -f ~/.cache/answerd/answerd.log
cat ~/.cache/answerd/latest.json
```

Скопируйте тестовый вопрос: в логе появятся `question` и `answer`, в `latest.json` — номер. Зажмите Ctrl — ответ появится в дате.

</details>

## Использование

| Действие | Результат |
|---|---|
| Скопировать текст вопроса | запрос в DeepSeek |
| Зажать **Ctrl** | показать ответ в дате |
| **Ctrl + Alt** | скриншот всего экрана → файл в inbox → запрос |
| `~/.local/bin/answerd screenshot` | скриншот без виджета (можно повесить на KDE-шорткат) |

Скриншоты **не копируются в буфер обмена**: они идут через файловую очередь `inbox` (см. «Как это работает»). Ручное копирование картинки из браузера по-прежнему обрабатывается через буфер.

## Как это работает

### Текст из буфера

```
Ctrl+C → буфер → wl-paste (350 мс, только text/plain) → фильтры/дедуп → DeepSeek → latest.json → виджет
```

- служебный MIME KDE (`application/x-kde-onlyReplaceEmpty`) игнорируется;
- `file://`-пути и текст короче `min_text_length` не отправляются.

### Скриншот (Ctrl+Alt)

```
Ctrl+Alt → answerd screenshot
  → latest.json = processing          (виджет показывает 00)
  → spectacle -o inbox/.tmp/shot-….png
  → атомарный rename в inbox/shot-….png
  → демон раз в 200 мс забирает самый старый файл
  → DeepSeek Vision → latest.json → виджет
```

- буфер обмена **не используется и не меняется** — поэтому снимок не появляется в буфере;
- файл удаляется из `inbox` после чтения; если демон не запущен, файлы ждут его старта;
- несколько снимков обрабатываются по очереди, без потерь;
- каждый снимок — отдельный запрос: дедупликации по SHA256 для `inbox` нет.

Виджет раз в 600 мс читает `latest.json`. При зажатом Ctrl: ответ → `MM/dd/<ответ>`, `processing` → `MM/dd/00`, ошибка/пусто → обычная дата.

## Настройки

Файл `~/.config/answerd/config.json`; обязателен только `api_key`.

| Поле | По умолчанию | Что делает |
|---|---|---|
| `api_key` | — | ключ DeepSeek (можно задать `DEEPSEEK_API_KEY`) |
| `base_url` | `https://api.deepseek.com` | адрес API |
| `model` | `deepseek-flash` | модель |
| `temperature` | `0` | детерминированность (не действует при `thinking: true`) |
| `max_tokens` | `16` | лимит ответа; автоматически поднимается при reasoning/поиске |
| `system_prompt` | встроенный | инструкция модели ("отвечай только цифрами") |
| `thinking` | `false` | reasoning перед ответом |
| `web_search` | `false` | серверный веб-поиск (модель решает сама) |
| `min_text_length` | `4` | игнорировать более короткий текст |
| `poll_interval_ms` | `350` | частота опроса буфера (inbox опрашивается раз в 200 мс) |
| `request_timeout_sec` | `30` | таймаут запроса (при `thinking`/поиске лучше 120) |
| `state_file` | `~/.cache/answerd/latest.json` | состояние для виджета |
| `history_file` | `~/.cache/answerd/history.jsonl` | история запросов |
| `image_dir` | `~/.cache/answerd/images` | сохранённые скриншоты |
| `inbox_dir` | `~/.cache/answerd/inbox` | очередь скриншотов от `answerd screenshot` |
| `log_file` | `~/.cache/answerd/answerd.log` | лог |

Ответ нормализуется: `1` → `01`, `1,3` / `1/3` → `13`; поддерживаются варианты 1–9. Если цифр нет — запрос считается ошибкой.

С `thinking: true` запрос к картинке может занимать 10–20 с — всё это время в дате висит `00`. Если это мешает, отключите `thinking` или `web_search`.

## Данные и приватность

- `history.jsonl` — вопросы, ответы, задержки;
- `images/` — скриншоты, ушедшие в модель;
- `inbox/` — временная очередь снимков (файл удаляется после обработки);
- текст вопроса или скриншот отправляется в DeepSeek;
- `answerd.log` — лог с короткими фрагментами вопросов и ответов;
- ключ хранится в `config.json` с правами `600`.

Очистить кэш:

```sh
rm -rf ~/.cache/answerd
```

Каталог создастся заново при следующем запросе.

## Если что-то не работает

| Симптом | Что проверить |
|---|---|
| Ответ не появляется | `pgrep -af answerd` и `tail -f ~/.cache/answerd/answerd.log` |
| `API error 401` | ключ API |
| `empty content` | при `thinking: true` нужен больший `max_tokens`; он поднимается автоматически |
| Ctrl+Alt не делает скриншот | в логе должно быть `screenshot: capturing` → `saved to inbox`, затем `inbox: image`; проверьте, что `spectacle -f -b -n -o /tmp/shot.png` создаёт файл |
| Скриншота нет в буфере | так и задумано: скриншоты идут в `inbox`, буфер не меняется |
| Долго висит `00` | идёт reasoning/поиск; с `thinking` картинка может обрабатываться 10–20 с |
| Снимок не обрабатывается | демон запущен? проверьте файлы в `~/.cache/answerd/inbox/` |
| Виджет показывает старое | переустановить виджет и заново добавить на панель |
| Тот же текст не обрабатывается | дедупликация по SHA256 для буфера: измените текст или перезапустите демон |

Для разработки: `qmllint` из `/usr/bin` — Qt 5 и падает на `pragma`. Используйте `/usr/lib/qt6/bin/qmllint` или `plasmawindowed answerd`.

## Удаление

```sh
./answerd.sh --uninstall          # оставить конфиг и кэш
./answerd.sh --uninstall --purge  # удалить всё
```

## Идеи

Нереализованные идеи и результаты их проверки — в [IDEAS.md](IDEAS.md).
