package service

import (
	"context"
	"log"
	"time"
)

// StartScheduler запускает ежедневные задания и поллер лайв матчей.
// 13:00 Алматы — синк → финальная таблица
// 20:00 Алматы — синк → расписание матчей
func StartScheduler(notif *NotificationService, sync *SyncService) {
	go runDaily(13, 0, almatyLoc, "results", func() {
		ctx := context.Background()
		syncAll(ctx, sync)
		notif.SendDailyResults(ctx)
	})
	go runDaily(20, 0, almatyLoc, "matches", func() {
		ctx := context.Background()
		syncAll(ctx, sync)
		notif.SendDailyMatches(ctx)
	})
	go runLiveSync(sync)
}

func syncAll(ctx context.Context, sync *SyncService) {
	if _, err := sync.SyncWC2026(ctx); err != nil {
		log.Printf("sync: wc2026: %v", err)
	}
	if _, err := sync.SyncCLFinal(ctx); err != nil {
		log.Printf("sync: cl-final: %v", err)
	}
}

// runLiveSync polls for IN_PLAY matches only during the active window (20:00–13:00 Almaty).
// During the quiet window (13:00–20:00) it sleeps until 20:00.
// When matches are in play it syncs every 2 minutes, otherwise every 5 minutes.
func runLiveSync(sync *SyncService) {
	for {
		if wait := quietWindowSleep(); wait > 0 {
			log.Printf("live-sync: quiet window, sleeping %s", wait.Round(time.Minute))
			time.Sleep(wait)
			continue
		}

		ctx := context.Background()
		inPlay, err := sync.matches.GetInPlay(ctx)
		if err != nil {
			log.Printf("live-sync: check in-play: %v", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		if len(inPlay) > 0 {
			log.Printf("live-sync: %d match(es) in play, syncing", len(inPlay))
			if _, err := sync.SyncWC2026(ctx); err != nil {
				log.Printf("live-sync: wc2026: %v", err)
			}
			if _, err := sync.SyncCLFinal(ctx); err != nil {
				log.Printf("live-sync: cl-final: %v", err)
			}
			time.Sleep(2 * time.Minute)
		} else {
			time.Sleep(5 * time.Minute)
		}
	}
}

// quietWindowSleep returns how long to sleep if we're in the quiet window (13:00–20:00 Almaty).
// Returns 0 if we're outside the quiet window and should poll normally.
func quietWindowSleep() time.Duration {
	now := time.Now().In(almatyLoc)
	h, m := now.Hour(), now.Minute()
	totalMin := h*60 + m

	quietStart := 13 * 60 // 13:00
	quietEnd := 20 * 60   // 20:00

	if totalMin < quietStart || totalMin >= quietEnd {
		return 0
	}

	next20 := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, almatyLoc)
	return time.Until(next20)
}

func runDaily(hour, min int, loc *time.Location, name string, fn func()) {
	for {
		now := time.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}

		log.Printf("scheduler: %s next run at %s", name, next.Format(time.RFC3339))
		time.Sleep(time.Until(next))

		log.Printf("scheduler: running %s", name)
		fn()
	}
}
