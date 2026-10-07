# Homebrew и релизы formatfast

Два независимых Git-репозитория:

- `leadfact/format-fast` — исходники, тесты, сборка и GitHub Releases.
- `leadfact/homebrew-format-fast` — `Formula/formatfast.rb` и инструкция установки.

Локальная структура:

```text
/Users/dnarek/study/golang/formatfast/
├── format-fast/
└── homebrew-format-fast/
```

Общий каталог не является отдельным репозиторием. У каждого проекта свой `.git`.
Имя исполняемого файла и формулы — `formatfast`.

## Установка стабильной версии

```sh
brew install leadfact/format-fast/formatfast
formatfast --version
formatfast '{"message":"hello\nworld"}' --extract message
```

Homebrew автоматически подключит `leadfact/homebrew-format-fast` и установит
готовый бинарник опубликованного релиза. Флаг `--HEAD` не требуется.
Для обновления:

```sh
brew update
brew upgrade leadfact/format-fast/formatfast
```

Если раньше tap был подключён к `leadfact/format-fast` через явный URL,
переключи его remote:

```sh
brew tap --custom-remote leadfact/format-fast https://github.com/leadfact/homebrew-format-fast.git
brew update
```

## Стабильный релиз

Workflow `.github/workflows/release.yml` основного проекта запускается при отправке
тега `vX.Y.Z`. Он выполняет тесты с race detector, собирает бинарники для macOS/Linux
на arm64/amd64, создаёт архивы, `checksums.txt` и `formatfast.rb`, проверяет
Linux-бинарник и синтаксис формулы, затем создаёт **черновик GitHub Release**.

Например, после коммита и отправки исходников в `main`:

```sh
cd /Users/dnarek/study/golang/formatfast/format-fast
git tag v0.3.0
git push origin v0.3.0
```

Дождись успешного workflow, открой GitHub → Releases, проверь черновик и нажми
Publish release, оставив Set as a pre-release выключенным.

Теперь обнови формулу **в tap**, используя файл из опубликованного релиза:

```sh
cd /Users/dnarek/study/golang/formatfast/homebrew-format-fast
curl --fail --location --output Formula/formatfast.rb \
  https://github.com/leadfact/format-fast/releases/download/v0.3.0/formatfast.rb
ruby -c Formula/formatfast.rb
git diff -- Formula/formatfast.rb
git add Formula/formatfast.rb
git commit -m "Update formatfast to 0.3.0"
git push origin main
```

Для следующей версии замени `0.3.0` во всех командах на её номер.
Workflow не коммитит и не отправляет изменения в tap автоматически.
Не используй локальную формулу с архивами, пересобранными в CI: контрольные суммы
должны соответствовать реально опубликованным файлам.

После обновления tap пользователю достаточно:

```sh
brew install leadfact/format-fast/formatfast
formatfast --version
```

Стабильная формула выбирает готовый бинарник по ОС и архитектуре, проверяет SHA-256;
Go для установки не нужен. Последующие обновления:

```sh
brew update
brew upgrade leadfact/format-fast/formatfast
```

Если уже установлена HEAD-версия, для перехода на стабильную:

```sh
brew update
brew uninstall leadfact/format-fast/formatfast
brew install leadfact/format-fast/formatfast
```

## Локальная проверка сборки релиза

Из каталога `format-fast`:

```sh
make test
make release VERSION=0.3.0
ruby -c dist/formatfast.rb
ruby -c ../homebrew-format-fast/Formula/formatfast.rb
```

`make release` создаёт только локальные файлы в игнорируемом `dist/`.
Команда не создаёт теги, коммиты и GitHub Releases. Версия встраивается через linker
`-X`. Архивы содержат только бинарник, без логов и `.env`.

Для создания черновика workflow использует стандартный `GITHUB_TOKEN` с
`contents: write`. Для общедоступной установки оба репозитория и релизные файлы
должны быть публичными.

Документация: https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap
