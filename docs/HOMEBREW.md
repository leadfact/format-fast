# Homebrew и релизы formatfast

Репозиторий: `leadfact/format-fast`. Имя исполняемого файла и формулы: `formatfast`.
Сам репозиторий используется как tap: отдельный `homebrew-*` репозиторий не требуется.
Поскольку название GitHub-репозитория не начинается с `homebrew-`, при первом `tap`
необходимо явно передать URL.

## До первого стабильного релиза

После публикации текущих изменений в `main`:

```sh
brew tap leadfact/format-fast https://github.com/leadfact/format-fast.git
brew install --HEAD leadfact/format-fast/formatfast
formatfast '{"message":"hello\nworld"}' --extract message
```

Начальная `Formula/formatfast.rb` собирает код из `main`. Homebrew сам устанавливает
Go как build-зависимость. Это работоспособный вариант без релизных архивов и без
выдуманных ссылок/контрольных сумм на ещё не опубликованные файлы.

Для обновления установки из `main`:

```sh
brew update
brew upgrade --fetch-HEAD leadfact/format-fast/formatfast
```

## Стабильные релизы с готовыми бинарниками

Настроен workflow `.github/workflows/release.yml`. Отправка тега `vX.Y.Z` запускает:

1. Тесты с race detector.
2. Сборку `CGO_ENABLED=0` для macOS arm64/amd64 и Linux arm64/amd64.
3. Упаковку архивов `formatfast_X.Y.Z_OS_ARCH.tar.gz`.
4. Создание `checksums.txt` и `formatfast.rb` с SHA-256 этих же архивов.
5. Проверку Linux-бинарника и синтаксиса формулы.
6. Создание **черновика GitHub Release** с перечисленными файлами.

Для релиза `0.3.0` после коммита и отправки изменений в `main`:

```sh
git tag v0.3.0
git push origin v0.3.0
```

Открой GitHub → Releases, проверь черновик и опубликуй его. Затем скачай формулу
**именно из опубликованного релиза** и замени начальную формулу в основной ветке:

```sh
gh release download v0.3.0 --repo leadfact/format-fast \
  --pattern formatfast.rb --dir Formula --clobber
```

Проверь diff `Formula/formatfast.rb`, закоммить и отправь этот файл в `main`.
Workflow не коммитит в основную ветку автоматически. Повторяй этот шаг для каждой
новой стабильной версии, чтобы `brew upgrade` видел свежие URL и контрольные суммы.
Не копируй локальную формулу, если архивы релиза были заново собраны на CI: хеши
должны соответствовать **реально опубликованным** архивам.

Теперь пользователю нужны команды:

```sh
brew tap leadfact/format-fast https://github.com/leadfact/format-fast.git
brew install leadfact/format-fast/formatfast
formatfast --version
```

Go при установке стабильной бинарной версии не нужен. Формула выберет нужный архив
по ОС и архитектуре и проверит SHA-256. Для обновления:

```sh
brew update
brew upgrade leadfact/format-fast/formatfast
```

Если раньше устанавливалась HEAD-версия, после появления стабильной формулы:

```sh
brew update
brew uninstall leadfact/format-fast/formatfast
brew install leadfact/format-fast/formatfast
```

## Локальная проверка релиза

```sh
make test
make release VERSION=0.3.0
ruby -c Formula/formatfast.rb
ruby -c dist/formatfast.rb
```

Команда `make release` создаёт только локальные файлы в игнорируемом `dist/`;
она не создаёт теги, коммиты и GitHub Releases. Версия записывается в каждый бинарник
через Go linker `-X`. Архивы содержат только исполняемый файл, без логов и `.env`.

В Actions используется стандартный `GITHUB_TOKEN` с `contents: write` для создания
черновика в этом же репозитории. Отдельный токен другого репозитория не требуется.
Для общедоступной установки репозиторий и релизные файлы должны быть публичными;
для приватного репозитория потребуется доступ GitHub у устанавливающего пользователя.

Существующие файлы Homebrew подготовлены локально; пока изменения и релиз не
опубликованы, команды установки из GitHub не доставят эту новую версию.

Документация Homebrew:
- https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap
- https://docs.brew.sh/Formula-Cookbook
