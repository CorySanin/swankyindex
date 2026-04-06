package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFs embed.FS

type (
	Download struct {
		Path         string
		Filename     string
		AccessDomain string
		UserAgent    string
		Timestamp    time.Time
	}

	Storage struct {
		db *sql.DB
	}

	migration struct {
		version   int
		statement string
	}

	Totals struct {
		All    int
		Recent int
	}
)

func (store *Storage) IncrementDownload(params Download) {
	if _, err := store.db.Exec("INSERT INTO downloads (Path,Filename,Timestamp) VALUES (?, ?, ?)", params.Path, params.Filename, time.Now()); err != nil {
		log.Printf("Failed to insert download: %v", err)
	}
}

func (store *Storage) GetTotalsByPath(path string, c chan map[string]Totals) {
	separator := string(os.PathSeparator)
	m, err := store.getTotalsByPath(strings.TrimSuffix(path, separator) + separator)
	if err != nil {
		c <- nil
		log.Printf("Failed to get totals for path %s: %v", path, err)
		return
	}
	c <- m
}

func (store *Storage) getTotalsByPath(path string) (map[string]Totals, error) {

	totalMap := make(map[string]Totals)

	t, err := store.getTotalsAllByPath(path)
	if err != nil {
		return nil, err
	}
	for t.Next() {
		var filename string
		var count int
		if err := t.Scan(&filename, &count); err != nil {
			return nil, err
		}
		totalMap[filename] = Totals{
			All: count,
		}
	}

	t, err = store.getTotalsRecentByPath(path, time.Now().Add(time.Hour*24*-3))
	if err != nil {
		return nil, err
	}
	for t.Next() {
		var filename string
		var count int
		if err := t.Scan(&filename, &count); err != nil {
			return nil, err
		}
		var existing = totalMap[filename]
		existing.Recent = count
		totalMap[filename] = existing
	}

	return totalMap, nil
}

func (store *Storage) getTotalsAllByPath(path string) (*sql.Rows, error) {
	result, err := store.db.Query("SELECT Filename, count() as Count FROM downloads WHERE Path = ? GROUP BY Filename", path)
	if err != nil {
		return nil, fmt.Errorf("Failed to get total downloads for path %s : %v", path, err)
	}
	return result, nil
}

func (store *Storage) getTotalsRecentByPath(path string, date time.Time) (*sql.Rows, error) {
	result, err := store.db.Query("SELECT Filename, count() as Count FROM downloads WHERE Path = ? AND Timestamp > ? GROUP BY Filename", path, date)
	if err != nil {
		return nil, fmt.Errorf("Failed to get recent downloads for path %s : %v", path, err)
	}
	return result, nil
}

func New(path string) Storage {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatalln(err)
	}

	applyMigrations(db)

	return Storage{
		db: db,
	}
}

func applyMigrations(db *sql.DB) {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		log.Fatalf("Failed to get user_version: %v", err)
	}
	migrations := getMigrations()
	for _, migration := range migrations {
		if migration.version <= version {
			continue
		}
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			log.Fatalf("Failed to create transaction for migration %d: %v", migration.version, err)
		}

		defer func() {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				log.Printf("Failed to roll back database migration: %v", err)
			}
		}()

		if _, err := tx.Exec(migration.statement); err != nil {
			log.Fatalf("Failed to perform DB migration %d: %v", migration.version, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`pragma user_version=%d`, migration.version)); err != nil {
			log.Fatalf("Failed to update DB version to %d: %v", migration.version, err)
		}
		if err := tx.Commit(); err != nil {
			log.Fatalf("Failed to commit migration %d: %v", migration.version, err)
		}
	}
}

func getMigrations() []migration {
	migrations := []migration{}
	files, err := migrationsFs.ReadDir("migrations")
	if err != nil {
		log.Fatalf("Failed to load embedded migrations: %v", err)
	}
	for _, v := range files {
		if v.IsDir() {
			continue
		}
		ver, err := filenameToVersion(v.Name())
		if err != nil {
			log.Fatal(err)
		}
		q, err := migrationsFs.ReadFile(path.Join("migrations", v.Name()))
		if err != nil {
			log.Fatalf("Failed to read embedded migration %s: %v", v.Name(), err)
		}
		migrations = append(migrations, migration{ver, string(q)})
	}
	return migrations
}

func filenameToVersion(filename string) (int, error) {
	var dashIndex = strings.Index(filename, "-")
	if dashIndex == -1 {
		return dashIndex, fmt.Errorf("SQL filename is not formatted correctly: %s", filename)
	}
	var versionTxt = filename[:dashIndex]
	i, err := strconv.Atoi(versionTxt)
	if err != nil {
		return -1, err
	}
	return i, nil
}
