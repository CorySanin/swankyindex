package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFs embed.FS

type (
	DownloadIndex struct {
		Path     string
		Filename string
	}

	Download struct {
		DownloadIndex
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

func (store *Storage) RemoveDownloads(dls []DownloadIndex) error {
	if len(dls) == 0 {
		return nil
	}

	tx, err := store.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cmd, err := tx.Prepare("DELETE FROM downloads WHERE Path = ? AND Filename = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer cmd.Close()

	for _, dl := range dls {
		if _, err := cmd.Exec(dl.Path, dl.Filename); err != nil {
			return fmt.Errorf("failed to execute delete: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (store *Storage) IncrementDownload(params Download) {
	if _, err := store.db.Exec("INSERT INTO downloads (Path,Filename,AccessDomain,UserAgent,Timestamp) VALUES (?, ?, ?, ?, ?)", params.Path, params.Filename, params.AccessDomain, params.UserAgent, time.Now()); err != nil {
		log.Printf("Failed to insert download: %v", err)
	}
}

func (store *Storage) GetTotalsByPath(path string, c chan map[string]Totals) {
	m, err := store.getTotalsByPath(path)
	if err != nil {
		c <- nil
		log.Printf("Failed to get totals for path %s: %v", path, err)
		return
	}
	c <- m
}

func (store *Storage) getTotalsByPath(path string) (map[string]Totals, error) {
	totalMap := make(map[string]Totals)

	t, err := store.db.Query("SELECT Filename, count() as Count FROM downloads WHERE Path = ? GROUP BY Filename", path)
	if err != nil {
		return nil, fmt.Errorf("failed to get total downloads for path %s: %v", path, err)
	}
	defer t.Close()
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

	recentDate := time.Now().Add(time.Hour * 24 * -3)
	rt, err := store.db.Query("SELECT Filename, count() as Count FROM downloads WHERE Path = ? AND Timestamp > ? GROUP BY Filename", path, recentDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent downloads for path %s: %v", path, err)
	}
	defer rt.Close()
	for rt.Next() {
		var filename string
		var count int
		if err := rt.Scan(&filename, &count); err != nil {
			return nil, err
		}
		total := totalMap[filename]
		total.Recent = count
		totalMap[filename] = total
	}

	return totalMap, nil
}

func (store *Storage) Optimize() error {
	log.Printf("Running DB optimize procedure...")
	if _, err := store.db.Exec(`
	PRAGMA analysis_limit=400;
	PRAGMA optimize;
	`); err != nil {
		log.Printf("Failed to exec DB optimization command: %v", err)
		return err
	}
	log.Printf("Done.")
	return nil
}

func New(path string) Storage {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatalf("Failed to open DB %s: %v", path, err)
	}

	db.SetMaxOpenConns(1)

	applyMigrations(db)

	if _, err := db.Exec(`
	PRAGMA journal_mode=WAL;
	PRAGMA synchronous=normal;
	PRAGMA temp_store = FILE;
	`); err != nil {
		log.Fatalf("Failed to initialize PRAGMA: %v", err)
	}

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
		q, err := migrationsFs.ReadFile(filepath.Join("migrations", v.Name()))
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
