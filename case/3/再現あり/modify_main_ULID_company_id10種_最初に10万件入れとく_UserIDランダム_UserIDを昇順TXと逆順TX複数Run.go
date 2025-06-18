package main

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DemoDeadlock struct {
	ID        string `gorm:"primaryKey"`
	CompanyID string `gorm:"index:idx_company_user,unique"`
	UserID    string `gorm:"index:idx_company_user,unique"`
	Data      string
}

var (
	companyULIDs  []string
	sharedEntropy = ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
)

func main() {
	runtime.GOMAXPROCS(10)
	dsn := "root:root@tcp(127.0.0.1:3306)/test_db?parseTime=true&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.Migrator().DropTable(&DemoDeadlock{}); err != nil {
		log.Fatalf("failed to drop table: %v", err)
	}

	if err := db.AutoMigrate(&DemoDeadlock{}); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("Seeding initial 100,000 records...")
	if err := seedInitialRecords(db); err != nil {
		log.Fatalf("failed to seed initial data: %v", err)
	}
	log.Println("Seeding completed.")

	numCompanies := 10
	companyULIDs = make([]string, numCompanies)
	for i := 0; i < numCompanies; i++ {
		companyULIDs[i] = ulid.MustNew(ulid.Timestamp(time.Now()), sharedEntropy).String()
		time.Sleep(2 * time.Millisecond)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(numCompanies)

	for i := 0; i < numCompanies; i++ {
		go func(cid int) {
			defer wg.Done()
			<-start
			log.Printf("Starting Tx for Company %d", cid+1)
			err := insertBatch(db, companyULIDs[cid])
			if err != nil {
				log.Printf("Tx for Company %d failed: %v", cid+1, err)
			} else {
				log.Printf("Tx for Company %d completed", cid+1)
			}
		}(i)
	}

	time.Sleep(2 * time.Second)
	close(start)
	wg.Wait()
	fmt.Println("done")
}

func seedInitialRecords(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < 100; i++ {
			records := make([]*DemoDeadlock, 0, 1000)
			for j := 0; j < 1000; j++ {
				id := ulid.MustNew(ulid.Timestamp(time.Now()), sharedEntropy).String()
				companyULID := ulid.MustNew(ulid.Timestamp(time.Now()), sharedEntropy).String()
				userULID := ulid.MustNew(ulid.Timestamp(time.Now()), sharedEntropy).String()
				records = append(records, &DemoDeadlock{
					ID:        id,
					CompanyID: companyULID,
					UserID:    userULID,
					Data:      fmt.Sprintf("INIT-DATA-%d-%d", i, j),
				})
			}
			if err := tx.CreateInBatches(records, 1000).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func insertBatch(db *gorm.DB, companyULID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		timestamp := time.Now().UnixNano()
		for batch := 0; batch < 20; batch++ {
			records := make([]*DemoDeadlock, 0, 1000)
			for i := 0; i < 1000; i++ {
				id := ulid.MustNew(ulid.Timestamp(time.Now()), sharedEntropy).String()
				userULID := ulid.MustNew(ulid.Timestamp(time.Now()), sharedEntropy).String()
				records = append(records, &DemoDeadlock{
					ID:        id,
					CompanyID: companyULID,
					UserID:    userULID,
					Data:      fmt.Sprintf("DATA-%d-%d", timestamp, rand.Intn(99999)),
				})
			}

			// 常に昇順でロック順序を統一
			sort.Slice(records, func(i, j int) bool {
				return records[i].UserID < records[j].UserID
			})

			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "company_id"}, {Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"data"}),
			}).CreateInBatches(records, 1000).Error; err != nil {
				return err
			}

			time.Sleep(50 * time.Millisecond)
		}
		return nil
	})
}