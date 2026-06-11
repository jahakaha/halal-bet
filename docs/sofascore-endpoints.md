# Sofascore RapidAPI Endpoints

Base URL: `https://sofascore.p.rapidapi.com`
Headers: `x-rapidapi-key`, `x-rapidapi-host: sofascore.p.rapidapi.com`

## WC2026
- tournamentId: 16
- seasonId: 58210

## Endpoints

### Все матчи WC2026 (статус + счёт)
GET /tournaments/get-matches?tournamentId=16&seasonId=58210&pageIndex=0

### Детали матча (счёт)
GET /matches/detail?matchId={id}

### Инциденты матча (голы, красные, пенальти, автоголы)
GET /matches/get-incidents?matchId={id}

### Live матчи (все виды спорта)
GET /tournaments/get-live-events?sport=football

### Список турниров по категории
GET /tournaments/list?categoryId={id}

### Список категорий
GET /categories/list?sport=football
- World = categoryId 1468

### Сезоны турнира
GET /tournaments/get-seasons?tournamentId={id}
