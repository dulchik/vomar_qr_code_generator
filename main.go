package main

import (
	cryptoRand "crypto/rand"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/mdp/qrterminal/v3"
	_ "modernc.org/sqlite"
)

const defaultLen = 10
const defaultAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func mustDB(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS codes(
		code TEXT PRIMARY KEY,
		created_at INTEGER NOT NULL
	)`)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func randCode(n int, alphabet string) (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < n; i++ {
		r, err := cryptoRand.Int(cryptoRand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[r.Int64()])
	}
	return b.String(), nil
}

func insertCode(db *sql.DB, code string) error {
	_, err := db.Exec(`INSERT INTO codes(code, created_at) VALUES(?, ?)`,
		code, time.Now().Unix())
	return err
}

func generateUnique(db *sql.DB, n int, alphabet string) (string, error) {
	for {
		code, err := randCode(n, alphabet)
		if err != nil {
			return "", err
		}
		if err := insertCode(db, code); err == nil {
			return code, nil
		}
		// conflict → try again
	}
}

func writeQRPNG(code, dir string, size int) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(dir, code+".png")
	if err := qrcode.WriteFile(code, qrcode.Medium, size, out); err != nil {
		return "", err
	}
	return out, nil
}

func showQRCLI(code string) {
	fmt.Println("\n🧾 QR Code Preview:")
	config := qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    os.Stdout,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
		QuietZone: 1,
	}
	qrterminal.GenerateWithConfig(code, config)
	fmt.Println() // spacing
}

func main() {
	var (
		dbPath   = flag.String("db", "codes.db", "SQLite path")
		dir      = flag.String("dir", "qr_codes", "output directory for PNGs")
		batch    = flag.Int("batch", 1, "how many fresh codes to generate")
		length   = flag.Int("len", defaultLen, "code length")
		alphabet = flag.String("alphabet", defaultAlphabet, "characters to use")
		size     = flag.Int("size", 256, "QR image size in px")
		addUsed  = flag.String("add-used", "", "record an existing code")
	)
	flag.Parse()

	db := mustDB(*dbPath)
	defer db.Close()

	if *addUsed != "" {
		code := strings.ToUpper(strings.TrimSpace(*addUsed))
		if err := insertCode(db, code); err == nil {
			fmt.Println("Recorded used code:", code)
		} else {
			fmt.Println("Code already recorded:", code)
		}
	}

	for i := 0; i < *batch; i++ {
		code, err := generateUnique(db, *length, *alphabet)
		if err != nil {
			log.Fatal(err)
		}
		path, err := writeQRPNG(code, *dir, *size)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\n✅ Generated code: %s\n📁 Saved: %s\n", code, path)
		showQRCLI(code)
	}
}

