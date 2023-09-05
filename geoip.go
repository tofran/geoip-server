package main

import (
	"compress/gzip"
	"fmt"
	"github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/oschwald/geoip2-golang"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"
	"strings"
)

const URL_TEMPLATE string = "https://updates.maxmind.com/geoip/databases/%s/update"

type geoResponseStruct struct {
	IP		  string  `json:"ip"`
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	Continent   string  `json:"continent"`
	StateCode   string  `json:"region_code"`
	StateName   string  `json:"region_name"`
	CityName	string  `json:"city"`
	PostalCode  string  `json:"zip_code"`
	TimeZone	string  `json:"time_zone"`
	Latitude	float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	MetroCode   int	 `json:"metro_code"`
}

type maxmind struct {
	mutex sync.RWMutex
	db	*geoip2.Reader
}

var m maxmind

func main() {
	var (
		bindIP			 string
		bindPort		   string
		prefix			 string
		license			string
		accountId		  string
		updateInterval	 int
		edition			string
		allowedOrigins	 []string
	)

	// TODO: add environment variable configuration
	pflag.StringVarP(&license, "license", "l", "", "Required: Sign up and generate this in the Maxmind website")
	pflag.StringVarP(&accountId, "account-id", "a", "0", "Required: Sign up and generate this in the Maxmind website")
	pflag.StringVarP(&bindIP, "bindip", "b", "0.0.0.0", "The ip address to bind to")
	pflag.StringVarP(&bindPort, "port", "p", "8080", "Port to listen on")
	pflag.IntVarP(&updateInterval, "update-interval", "u", 24, "Intervals in hours to check for database updates")
	pflag.StringVarP(&edition, "edition", "e", "GeoLite2-City", "edition of database to download")
	pflag.StringVarP(&prefix, "route-prefix", "r", "/geoip", "route prefix for geoip service, must not be empty")
	pflag.StringSliceVarP(&allowedOrigins, "allowed-origins", "o", []string{}, "Origins for the Access-Control-Allow-Origin header")
	pflag.Parse()

	db, err := downloadDatabase(edition, accountId, license)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	log.Info().Msg("Download finished")

	err = reload(db)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	defer m.db.Close()

	go func() {
		for {
			time.Sleep(time.Duration(updateInterval) * time.Hour)
			db, err := downloadDatabase(edition, accountId, license)
			if err != nil {
				log.Error().Err(err).Msg("Downloading update failed")
				continue
			}
			log.Info().Msg("Download finished")
			err = reload(db)
			if err != nil {
				log.Error().Err(err).Msg("Reload failed")
			}
		}
	}()

	router := httprouter.New()
	router.GET(prefix, headersMiddleware(geoHandler, allowedOrigins))
	router.GET(prefix + "/:ip", headersMiddleware(geoHandler, allowedOrigins))
	router.GET("/healthz", healthCheckHandler)

	log.Fatal().Err(http.ListenAndServe(bindIP+":"+bindPort, router)).Msg("")
}

func healthCheckHandler(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	return
}

func originIsAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		return true
	}

	for _, allowedOrigin := range allowedOrigins {
		if origin == allowedOrigin {
			return true
		}
	}
	return false
}

func headersMiddleware(next httprouter.Handle, allowedOrigins []string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")

		origin := r.Header.Get("Origin")
		if originIsAllowed(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Methods", "GET")
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		next(w, r, ps)
	}
}

func errResponse(w http.ResponseWriter, statusCode int, errStr string) {
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(`{"error": "` + errStr + `"}`))
	if err != nil {
		log.Error().Err(err).Msg("")
	}
}

func geoResponse(w http.ResponseWriter, geo geoResponseStruct) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	j, err := json.Marshal(geo)
	if err != nil {
		errResponse(w, http.StatusInternalServerError, "")
		return
	}
	_, err = w.Write(j)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
}

func getClientIP(request *http.Request) string {
	ip := request.Header.Get("X-Real-IP")

	if ip == "" {
		ip = request.Header.Get("X-Forwarded-For")
	}

	if ip == "" {
		ip = request.RemoteAddr
	}

	parts := strings.Split(ip, ",")

	if len(parts) == 0 {
		return ""
	}
	
	firstElement := strings.TrimSpace(parts[0])
	return firstElement
}

func geoHandler(w http.ResponseWriter, request *http.Request, ps httprouter.Params) {
	ipStr := ps.ByName("ip")

	if ipStr == "" {
		ipStr = getClientIP(request)
	}
	
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Info().Msg(fmt.Sprintf("Invalid IP: '%s'", ipStr))
		errResponse(w, http.StatusBadRequest, "Invalid IP address")
		return
	}
	
	log.Info().Msg(fmt.Sprintf("Looking up IP '%s'", ipStr))

	m.mutex.RLock()
	geo, err := m.db.City(ip)
	m.mutex.RUnlock()
	if err != nil {
		log.Err(err).Msg("Lookup error")
		errResponse(w, http.StatusInternalServerError, "Lookup error")
		return
	}

	stateName := ""
	stateCode := ""
	if len(geo.Subdivisions) > 0 {
		stateName = geo.Subdivisions[0].Names["en"]
		stateCode = geo.Subdivisions[0].IsoCode
	}
	resp := geoResponseStruct{
		IP:		  ipStr,
		CountryCode: geo.Country.IsoCode,
		CountryName: geo.Country.Names["en"],
		Continent:   geo.Continent.Names["en"],
		StateCode:   stateCode,
		StateName:   stateName,
		CityName:	geo.City.Names["en"],
		PostalCode:  geo.Postal.Code,
		Latitude:	geo.Location.Latitude,
		Longitude:   geo.Location.Longitude,
		TimeZone:	geo.Location.TimeZone,
	}

	geoResponse(w, resp)
}

func downloadDatabase(edition string, accountId string, license string) ([]byte, error) {
	url := fmt.Sprintf(URL_TEMPLATE, edition)

	log.Info().Msg(fmt.Sprintf("Starting database download (edition: '%s')", edition))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(accountId, license)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tempBytes, err := ioutil.ReadAll(gzr)
	return tempBytes, nil
}

func reload(newDB []byte) error {
	newReader, err := geoip2.FromBytes(newDB)
	if err != nil {
		return err
	}
	m.mutex.Lock()
	m.db = newReader
	m.mutex.Unlock()
	return nil
}
