//go:build integration
// +build integration

package integration

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrency_ParallelWrites(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	t.Run("concurrent_creates_no_conflict", func(t *testing.T) {
		const concurrency = 10
		var wg sync.WaitGroup
		errors := make(chan error, concurrency)
		ids := make(chan string, concurrency)

		for i := range concurrency {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				book := Book{
					Name:        "Concurrent Book",
					Author:      "Author",
					Pages:       100 + idx,
					PublishedAt: time.Now(),
					ISBN:        string(rune('0'+idx)) + "00",
					InStock:     true,
				}
				err := bookSchema.Put(fixture.Ctx, &book)
				if err != nil {
					errors <- err
					return
				}
				ids <- book.ID
			}(i)
		}

		wg.Wait()
		close(errors)
		close(ids)

		for err := range errors {
			t.Errorf("Concurrent write error: %v", err)
		}

		for id := range ids {
			bookSchema.Delete(fixture.Ctx, id)
		}
	})

	t.Run("concurrent_updates_same_object", func(t *testing.T) {
		book := Book{
			Name: "Update Target", Author: "Author", Pages: 100,
			PublishedAt: time.Now(), ISBN: "update", InStock: true,
		}
		bookSchema.Put(fixture.Ctx, &book)
		defer bookSchema.Delete(fixture.Ctx, book.ID)

		const concurrency = 5
		var wg sync.WaitGroup
		successes := make(chan bool, concurrency)

		for i := range concurrency {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_, err := bookSchema.Update(fixture.Ctx, book.ID, func(b *Book) error {
					b.Pages = 100 + idx
					time.Sleep(10 * time.Millisecond) // simulate some processing time
					return nil
				})
				successes <- (err == nil)
			}(i)
		}

		wg.Wait()
		close(successes)

		successCount := 0
		for success := range successes {
			if success {
				successCount++
			}
		}

		if successCount == 0 {
			t.Error("All concurrent updates failed, expected at least one to succeed")
		}
		t.Logf("%d/%d concurrent updates succeeded", successCount, concurrency)
	})
}

func TestConcurrency_ReadWhileWrite(t *testing.T) {
	fixture := Setup(t)
	bookSchema := fixture.CreateBookSchema(t)

	book := Book{
		Name: "Read While Write", Author: "Author", Pages: 100,
		PublishedAt: time.Now(), ISBN: "rww", InStock: true,
	}
	bookSchema.Put(fixture.Ctx, &book)
	defer bookSchema.Delete(fixture.Ctx, book.ID)

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-stopChan:
				return
			default:
				bookSchema.Update(fixture.Ctx, book.ID, func(b *Book) error {
					b.Pages = 100 + i
					return nil
				})
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				select {
				case <-stopChan:
					return
				default:
					_, err := bookSchema.Get(fixture.Ctx, book.ID)
					if err != nil {
						t.Errorf("Read error during concurrent write: %v", err)
					}
					time.Sleep(25 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	close(stopChan)
}
