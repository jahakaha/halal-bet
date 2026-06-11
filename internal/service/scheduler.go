package service

import (
	"context"
	"log"
	"time"
)

// StartScheduler запускает ежедневные задания и поллер лайв матчей.
// 12:00 Алматы — синк → результаты вчера + матчи завтра
func StartScheduler(notif *NotificationService, sync *SyncService) {
	go runDaily(12, 0, almatyLoc, "results", func() {
		ctx := context.Background()
		syncAll(ctx, sync)
		notif.SendDailyResults(ctx, time.Now())
	})
	go runDaily(12, 5, almatyLoc, "matches", func() {
		ctx := context.Background()
		notif.SendDailyMatches(ctx, time.Now())
	})
	go runLiveSync(sync)
}

func syncAll(ctx context.Context, sync *SyncService) {
	if _, err := sync.SyncWC2026(ctx); err != nil {
		log.Printf("sync: wc2026: %v", err)
	}
}

// runLiveSync polls continuously outside the quiet window (12:10–21:00 Almaty).
// Always syncs so that match statuses transition to IN_PLAY correctly.
// When matches are in play it syncs every 2 minutes, otherwise every 5 minutes.
func runLiveSync(sync *SyncService) {
	for {
		if wait := quietWindowSleep(); wait > 0 {
			log.Printf("live-sync: quiet window, sleeping %s", wait.Round(time.Minute))
			time.Sleep(wait)
			continue
		}

		ctx := context.Background()
		log.Printf("live-sync: syncing")
		if _, err := sync.SyncWC2026(ctx); err != nil {
			log.Printf("live-sync: wc2026: %v", err)
		}

		inPlay, err := sync.matches.GetInPlay(ctx)
		if err != nil {
			log.Printf("live-sync: check in-play: %v", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		if len(inPlay) > 0 {
			log.Printf("live-sync: %d match(es) in play, next sync in 2m", len(inPlay))
			time.Sleep(2 * time.Minute)
		} else {
			log.Printf("live-sync: no matches in play, next sync in 5m")
			time.Sleep(5 * time.Minute)
		}
	}
}

// quietWindowSleep returns how long to sleep if we're in the quiet window (12:10–21:00 Almaty).
// Returns 0 if we're outside the quiet window and should poll normally.
func quietWindowSleep() time.Duration {
	now := time.Now().In(almatyLoc)
	h, m := now.Hour(), now.Minute()
	totalMin := h*60 + m

	quietStart := 12*60 + 10 // 12:10
	quietEnd := 21 * 60       // 21:00

	if totalMin < quietStart || totalMin >= quietEnd {
		return 0
	}

	next21 := time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, almatyLoc)
	return time.Until(next21)
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
