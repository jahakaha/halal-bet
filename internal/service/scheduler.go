package service

import (
	"context"
	"log"
	"time"
)

// StartScheduler запускает ежедневные задания и поллер лайв матчей.
// 12:00 Алматы — синк → результаты вчера + матчи завтра
func StartScheduler(notif *NotificationService, sync *SyncService) {
	go runDaily(11, 55, almatyLoc, "events", func() {
		ctx := context.Background()
		if n, err := sync.SyncMatchEvents(ctx); err != nil {
			log.Printf("scheduler: events: %v", err)
		} else {
			log.Printf("scheduler: events synced for %d matches", n)
		}
	})
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
	go runPreMatchNotify(notif)

	// Backfill on startup: sync events for all finished matches that still have pending risky bets.
	go func() {
		ctx := context.Background()
		if n, err := sync.SyncMatchEvents(ctx); err != nil {
			log.Printf("startup: events backfill: %v", err)
		} else if n > 0 {
			log.Printf("startup: backfilled events for %d matches", n)
		}
	}()
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

		if pending, _ := sync.matches.GetFinishedForEventSync(ctx); len(pending) > 0 {
			log.Printf("live-sync: %d match(es) need events, syncing sofascore", len(pending))
			if n, err := sync.SyncMatchEvents(ctx); err != nil {
				log.Printf("live-sync: events: %v", err)
			} else {
				log.Printf("live-sync: events synced for %d matches", n)
			}
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

// runPreMatchNotify sleeps until 1 hour before the first match of the day,
// then sends pre-match reminders per group showing who hasn't bet yet.
// If no matches today or already past the notify time, sleeps until next day.
func runPreMatchNotify(notif *NotificationService) {
	for {
		ctx := context.Background()
		now := time.Now().In(almatyLoc)
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, almatyLoc).UTC()
		dayEnd := dayStart.Add(24 * time.Hour)

		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, almatyLoc)

		matches, err := notif.matches.GetUpcoming(ctx, dayStart, dayEnd)
		if err != nil || len(matches) == 0 {
			log.Printf("pre-match: no matches today, sleeping until %s", tomorrow.Format(time.RFC3339))
			time.Sleep(time.Until(tomorrow))
			continue
		}

		notifyAt := matches[0].MatchDate.Add(-1 * time.Hour)
		if time.Now().After(notifyAt) {
			log.Printf("pre-match: notify time passed, sleeping until %s", tomorrow.Format(time.RFC3339))
			time.Sleep(time.Until(tomorrow))
			continue
		}

		log.Printf("pre-match: sleeping until %s (1h before first match)", notifyAt.In(almatyLoc).Format("15:04"))
		time.Sleep(time.Until(notifyAt))

		notif.SendPreMatchReminder(ctx, time.Now())

		now2 := time.Now().In(almatyLoc)
		nextDay := time.Date(now2.Year(), now2.Month(), now2.Day()+1, 0, 5, 0, 0, almatyLoc)
		time.Sleep(time.Until(nextDay))
	}
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
