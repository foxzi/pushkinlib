# Pushkinlib

Web-сервис для просмотра локальной библиотеки книг по INPX-индексу с поддержкой OPDS.

## Возможности

- 📚 **Парсинг INPX** - поддержка индексных файлов библиотек
- 🔍 **Полнотекстовый поиск** - по названию, автору, аннотации
- 🌐 **Web интерфейс** - современный SPA на Vue.js
- 📱 **Адаптивный дизайн** - работает на мобильных устройствах
- 📖 **OPDS каталог** - совместимость с читалками
- 🗂️ **Фильтрация** - по жанрам, сериям, авторам, годам
- ⚡ **Быстрая работа** - SQLite база данных с индексами

## Быстрый старт

### 1. Сборка

```bash
go build -o pushkinlib ./cmd/pushkinlib
```

### 2. Конфигурация

Скопируйте и настройте конфигурацию:

```bash
cp .env.example .env
```

Основные параметры в `.env`:
```env
PORT=9090
BOOKS_DIR=/path/to/books
INPX_PATH=/path/to/catalog.inpx
CATALOG_TITLE=Моя библиотека
```

### 3. Запуск

```bash
./pushkinlib
```

Сервис будет доступен по адресам:
- **Web интерфейс**: http://localhost:9090/
- **API**: http://localhost:9090/api/v1/books
- **OPDS каталог**: http://localhost:9090/opds

## Генерация каталога из книг

Если у вас есть папка с книгами, но нет INPX файла, используйте генератор каталога:

### 1. Сборка генератора

```bash
go build -o catalog-generator ./cmd/catalog-generator
```

### 2. Подготовка книг

Поместите книги в папку `./sample-data/books/`:
```
sample-data/books/
├── book1.fb2
├── book2.zip
├── book3.epub
└── subfolder/
    └── book4.fb2
```

### 3. Генерация каталога

```bash
./catalog-generator -books=./sample-data/books -name=my_catalog
```

Опции генератора:
- `-books` - папка с книгами (по умолчанию: `./sample-data/books`)
- `-output` - папка для результатов (по умолчанию: `./sample-data`)
- `-name` - имя каталога (по умолчанию: `generated_catalog`)
- `-prefix` - префикс для архивов (по умолчанию: `books`)
- `-max-books` - максимум книг в архиве (по умолчанию: 1000)
- `-formats` - форматы файлов (по умолчанию: `.fb2,.zip,.epub`)

### 4. Использование сгенерированного каталога

После генерации обновите `.env`:
```env
INPX_PATH=./sample-data/my_catalog.inpx
BOOKS_DIR=./sample-data
```

## Поддерживаемые форматы

### Извлечение метаданных
- **FB2** - полная поддержка метаданных
- **FB2.ZIP** - FB2 файлы в ZIP архивах
- **EPUB** - базовая поддержка (название, автор)

### Файлы каталога
- **INPX** - стандартный формат индексов
- **INP** - отдельные файлы индексов

## API

### Поиск книг
```http
GET /api/v1/books?q=запрос&limit=30&offset=0
```

Параметры:
- `q` - поисковый запрос
- `limit` - количество результатов (по умолчанию: 30)
- `offset` - смещение для пагинации
- `authors[]` - фильтр по авторам
- `series[]` - фильтр по сериям
- `genres[]` - фильтр по жанрам
- `year_from`, `year_to` - фильтр по годам
- `sort_by` - сортировка (`title`, `year`, `date_added`, `relevance`)
- `sort_order` - порядок (`asc`, `desc`)

### Получение книги
```http
GET /api/v1/books/{id}
```

## OPDS

OPDS каталог доступен по адресу `/opds` и поддерживает:

- **Навигацию** - по авторам, сериям, жанрам
- **Поиск** - совместим с OpenSearch
- **Пагинацию** - для больших каталогов
- **Скачивание** - прямые ссылки на файлы

### Настройка читалок

Добавьте в вашу читалку OPDS каталог:
```
http://your-server:9090/opds
```

Протестированные читалки:
- FBReader
- KyBook
- Bookari
- Moon+ Reader

## Разработка

### Структура проекта

```
pushkinlib/
├── cmd/
│   ├── pushkinlib/          # Основное приложение
│   └── catalog-generator/   # Генератор каталогов
├── internal/
│   ├── api/                 # HTTP API handlers
│   ├── auth/                # Аутентификация
│   ├── catalog/             # Генерация каталогов
│   ├── config/              # Конфигурация
│   ├── covers/              # Обработка обложек
│   ├── inpx/                # Парсинг INPX
│   ├── metadata/            # Извлечение метаданных
│   ├── opds/                # OPDS каталог
│   ├── search/              # Поиск и индексация
│   └── storage/             # База данных
├── web/static/              # Frontend (Vue.js SPA)
└── sample-data/             # Тестовые данные
```

### Тестирование

```bash
# Запуск тестов
go test ./...

# Тест парсера INPX
go test ./internal/inpx -v

# Генерация тестового каталога
./catalog-generator -books=./sample-data/books
```

## Лицензия

MIT License

## Вклад в проект

1. Fork проекта
2. Создайте feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменения (`git commit -m 'Add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Откройте Pull Request

## Поддержка

Если у вас возникли вопросы или проблемы, создайте issue в репозитории GitHub.