package main

import (
	"context"
	"encoding/json"
	"fmt"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jackc/pgx/v4"
	"log"
	"net/http"
	"os"
	"time"
)

var conn *pgx.Conn

const ENERGY_WAIT_MS = 15 * 60 * 1000
const ENERGY_VIDEO_WAIT_MS = 15 * 60 * 1000

type Info struct {
	Time    int64
	TimeUTC int64

	Energy int32
	Money  float32

	EnergyTimer      int64
	EnergyVideoTimer int64

	Bet1    int32
	Bet10   int32
	Bet100  int32
	Bet1000 int32
}

func getArg(req *http.Request, name string) string {
	args, ok := req.URL.Query()[name]

	if !ok || len(args[0]) < 1 {
		return ""
	}

	return args[0]
}

func getInfoHandler(w http.ResponseWriter, req *http.Request) {
	id := getArg(req, "id")

	if len(id) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var info Info

	row := conn.QueryRow(context.Background(), "SELECT energy, money, energy_timer, energy_video_timer, coalesce(bet1.bet, 0), coalesce(bet10.bet, 0), coalesce(bet100.bet, 0), coalesce(bet1000.bet, 0) FROM users LEFT JOIN bet1 ON users.id = bet1.id LEFT JOIN bet10 ON users.id = bet10.id LEFT JOIN bet100 ON users.id = bet100.id LEFT JOIN bet1000 ON users.id = bet1000.id WHERE users.id=$1", id)
	err := row.Scan(&info.Energy, &info.Money, &info.EnergyTimer, &info.EnergyVideoTimer, &info.Bet1, &info.Bet10, &info.Bet100, &info.Bet1000)

	if err == pgx.ErrNoRows {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	info.Time = time.Now().UnixNano() / 1000000

	infoJson, err := json.Marshal(info)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(infoJson))
}

func registerHandler(w http.ResponseWriter, req *http.Request) {
	id := getArg(req, "id")
	name := getArg(req, "name")

	if len(id) == 0 || len(name) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	_, err := conn.Exec(context.Background(), "INSERT INTO users (id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING", id, name)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "OK")
}

func headers(w http.ResponseWriter, req *http.Request) {
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}

func getEnergyHandler(w http.ResponseWriter, req *http.Request) {
	id := getArg(req, "id")

	if len(id) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var energyTimeMs int64

	row := conn.QueryRow(context.Background(), "select energy_timer from users where id=$1", id)
	err := row.Scan(&energyTimeMs)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	curTimeMs := time.Now().UnixNano() / 1000000

	if curTimeMs >= energyTimeMs {
		newEnergyTimer := curTimeMs + ENERGY_WAIT_MS
		_, err := conn.Exec(context.Background(), "UPDATE users SET energy = energy + 1, energy_timer = $2 where id=$1", id, newEnergyTimer)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%d", newEnergyTimer)
	} else {
		fmt.Fprintf(w, "WAIT")
	}

}

func main() {

	var err error
	conn, err = pgx.Connect(context.Background(), "postgres://tnqhqsdfdamjoa:47e01ba2a9d708fa5dcdf10653b64e7c588ca9eef2befdf3415a8eaee4d318d2@ec2-54-75-225-52.eu-west-1.compute.amazonaws.com:5432/d516nrb1815o1")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	//defer conn.Close(context.Background())

	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	http.HandleFunc("/info", getInfoHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/headers", headers)
	http.ListenAndServe(":"+port, nil)

	println("WORKING")
}
