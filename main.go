package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jackc/pgx/v4"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var conn *pgx.Conn

const ENERGY_WAIT_MS = 15 * 1 * 1000
const ENERGY_VIDEO_WAIT_MS = 15 * 1 * 1000

var maxBet1 = 0
var maxBet10 = 0
var maxBet100 = 0
var maxBet1000 = 0

var maxBet1Players = 0
var maxBet10Players = 0
var maxBet100Players = 0
var maxBet1000Players = 0

var totalPlayers = 0
var totalGames = 0
var totalMoney = 0
var totalEnergy = 0

type Info struct {
	Time int64

	Energy int32
	Money  float32

	TotalEnergy int32
	TotalMoney  float32

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
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var info Info

	row := conn.QueryRow(context.Background(), "SELECT energy, money, total_energy, total_money, energy_timer, energy_video_timer, coalesce(bet1.bet, 0), coalesce(bet10.bet, 0), coalesce(bet100.bet, 0), coalesce(bet1000.bet, 0) FROM users LEFT JOIN bet1 ON users.id = bet1.id LEFT JOIN bet10 ON users.id = bet10.id LEFT JOIN bet100 ON users.id = bet100.id LEFT JOIN bet1000 ON users.id = bet1000.id WHERE users.id=$1", id)
	err := row.Scan(&info.Energy, &info.Money, &info.TotalEnergy, &info.TotalMoney, &info.EnergyTimer, &info.EnergyVideoTimer, &info.Bet1, &info.Bet10, &info.Bet100, &info.Bet1000)

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
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	_, err := conn.Exec(context.Background(), "INSERT INTO users (id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING", id, name)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	totalPlayers++
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
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
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
		_, err := conn.Exec(context.Background(), "UPDATE users SET energy = energy + 1, total_energy = total_energy + 1, energy_timer = $2 where id=$1", id, newEnergyTimer)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		totalEnergy++
		fmt.Fprintf(w, "%d", newEnergyTimer)
	} else {
		fmt.Fprintf(w, "WAIT")
	}

}

func getVideoEnergyHandler(w http.ResponseWriter, req *http.Request) {
	id := getArg(req, "id")

	if len(id) == 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var energyVideoTimeMs int64

	row := conn.QueryRow(context.Background(), "select energy_video_timer from users WHERE id=$1", id)
	err := row.Scan(&energyVideoTimeMs)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	curTimeMs := time.Now().UnixNano() / 1000000

	if curTimeMs >= energyVideoTimeMs {
		newVideoEnergyTimer := curTimeMs + ENERGY_VIDEO_WAIT_MS
		_, err := conn.Exec(context.Background(), "UPDATE users SET energy = energy + 1, total_energy = total_energy + 1, energy_video_timer = $2 where id=$1", id, newVideoEnergyTimer)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		totalEnergy++
		fmt.Fprintf(w, "%d", newVideoEnergyTimer)
	} else {
		fmt.Fprintf(w, "WAIT")
	}

}

func getMaxBetHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "%d %d %d %d %d %d %d %d",
		maxBet1, maxBet10, maxBet100, maxBet1000, maxBet1Players, maxBet10Players, maxBet100Players, maxBet1000Players)
}

func betHandler(w http.ResponseWriter, req *http.Request) {
	id := getArg(req, "id")

	if len(id) == 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	game := getArg(req, "game")

	if len(game) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if _, err := strconv.Atoi(game); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var energy int

	row := conn.QueryRow(context.Background(), "SELECT energy FROM users WHERE id=$1", id)
	err := row.Scan(&energy)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if energy > 0 {

		_, err := conn.Exec(context.Background(), "UPDATE users SET energy = energy - 1 where id=$1", id)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		var bet int

		row = conn.QueryRow(context.Background(), "INSERT INTO bet"+game+" (id, bet) VALUES ($1, 1) ON CONFLICT (id) DO UPDATE SET bet = bet"+game+".bet + 1 RETURNING bet;", id)
		err = row.Scan(&bet)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if game == "1" {
			if bet > maxBet1 {
				maxBet1 = bet
				maxBet1Players = 0
			} else if bet == maxBet1 {
				maxBet1Players++
			}
		} else if game == "10" {
			if bet > maxBet10 {
				maxBet10 = bet
				maxBet10Players = 0
			} else if bet == maxBet10 {
				maxBet10Players++
			}
		} else if game == "100" {
			if bet > maxBet100 {
				maxBet100 = bet
				maxBet100Players = 0
			} else if bet == maxBet100 {
				maxBet100Players++
			}
		} else if game == "1000" {
			if bet > maxBet1000 {
				maxBet1000 = bet
				maxBet1000Players = 0
			} else if bet == maxBet1000 {
				maxBet1000Players++
			}
		}

		fmt.Fprintf(w, "%d %d %d %d %d %d %d %d %d",
			bet, maxBet1, maxBet10, maxBet100, maxBet1000, maxBet1Players, maxBet10Players, maxBet100Players, maxBet1000Players)

	} else {
		http.Error(w, http.StatusText(http.StatusNotAcceptable), http.StatusNotAcceptable)
		return
	}

}

func betLeaderboardHandler(w http.ResponseWriter, req *http.Request) {
	game := getArg(req, "game")

	if len(game) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if _, err := strconv.Atoi(game); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rows, err := conn.Query(context.Background(), "SELECT bet, name FROM bet"+game+" LEFT JOIN users ON bet1.id = users.id ORDER BY bet DESC LIMIT 100")

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	var resultBuf bytes.Buffer

	fmt.Fprintf(w, "%d %d\n", maxBet1, maxBet1Players)

	for rows.Next() {
		var bet int32
		var name string
		err = rows.Scan(&bet, &name)
		fmt.Fprintf(&resultBuf, "%d %s;", bet, name)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprint(w, resultBuf.String())

}

func getHistoryHandler(w http.ResponseWriter, req *http.Request) {
	rows, err := conn.Query(context.Background(), "SELECT win, time, name FROM winners LEFT JOIN users ON winners.id = users.id LIMIT 100")

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	var resultBuf bytes.Buffer

	fmt.Fprintf(w, "%d %d %d %d\n", totalGames, totalPlayers, totalEnergy, totalMoney)

	for rows.Next() {
		var win float32
		var name string
		var time int64
		err = rows.Scan(&win, &time, &name)
		fmt.Fprintf(&resultBuf, "%f %d %s;", win, time, name)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprint(w, resultBuf.String())

}

func getLeaderboardHandler(w http.ResponseWriter, req *http.Request) {
	rows, err := conn.Query(context.Background(), "SELECT total_energy, total_money, name FROM users ORDER BY total_money DESC LIMIT 100")

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	var resultBuf bytes.Buffer

	for rows.Next() {
		var energy int32
		var money float32
		var name string
		err = rows.Scan(&energy, &money, &name)
		fmt.Fprintf(&resultBuf, "%d %f %s;", energy, money, name)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprint(w, resultBuf.String())

}

func getStatistics() {
	conn.QueryRow(context.Background(), "SELECT coalesce(max(bet),0), count(*) FROM bet1 WHERE bet=(SELECT max(bet) FROM bet1)").Scan(&maxBet1, &maxBet1Players)
	conn.QueryRow(context.Background(), "SELECT coalesce(max(bet),0), count(*) FROM bet10 WHERE bet=(SELECT max(bet) FROM bet10)").Scan(&maxBet10, &maxBet10Players)
	conn.QueryRow(context.Background(), "SELECT coalesce(max(bet),0), count(*) FROM bet100 WHERE bet=(SELECT max(bet) FROM bet100)").Scan(&maxBet100, &maxBet100Players)
	conn.QueryRow(context.Background(), "SELECT coalesce(max(bet),0), count(*) FROM bet1000 WHERE bet=(SELECT max(bet) FROM bet100)").Scan(&maxBet1000, &maxBet1000Players)

	conn.QueryRow(context.Background(), "SELECT coalesce(count(*),0), coalesce(sum(total_money),0), coalesce(sum(total_energy),0) FROM users").Scan(&totalPlayers, &totalMoney, &totalEnergy)
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

	getStatistics()

	http.HandleFunc("/info", getInfoHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/getEnergy", getEnergyHandler)
	http.HandleFunc("/getVideoEnergy", getVideoEnergyHandler)
	http.HandleFunc("/getMaxBet", getMaxBetHandler)
	http.HandleFunc("/bet", betHandler)
	http.HandleFunc("/betLeaderboard", betLeaderboardHandler)
	http.HandleFunc("/getHistory", getHistoryHandler)
	http.HandleFunc("/getLeaderboard", getLeaderboardHandler)
	http.HandleFunc("/headers", headers)
	http.ListenAndServe(":"+port, nil)

	println("WORKING")
}
