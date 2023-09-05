# GeoIP Server

No bullshit blazing fast IP Geolocation API server.

## Usage

GET `/<ROUTE_PREFIX>/geoip/<IP_ADDRESS>` for querying a specific IP.
GET `/<ROUTE_PREFIX>/geoip` for querying by the requester IP.
GET `/healthz` simple health check.

Examples:

```sh
# Query with an IPv4
curl https://localhost:8080/geoip/50.19.0.1

# Query with an IPv6
curl http://localhost:8080/geoip/2a09:9280:1::61:48a4

# Query using the request IP
curl http://localhost:8080/geoip
# Or using the proxy IP (X-Real-IP or X-Forwarded-For)
curl http://localhost:8080/geoip/2a09:9280:1::61:48a4 --header 'X-Real-IP: 50.19.0.1'

# Check if the service is alive (empty response)
curl http://localhost:8080/healthz
```

Example response:

```json
{
   "ip": "2a09:9280:1::61:48a4",
   "country_code": "DE",
   "country_name": "Germany",
   "continent": "Europe",
   "region_code": "",
   "region_name": "",
   "city": "",
   "zip_code": "",
   "time_zone": "Europe/Berlin",
   "latitude": 51.2993,
   "longitude": 9.491,
   "metro_code": 0
}
```

## Starting the server

1. [Sign up for the GeoLite2 Free geolocation database by Maxmind databases](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)
1. Under the account, create a license (Account > Manage License Keys)


### From the source

1. Build : `go build geoip.go`
1. Run `./geoip.go`. Ex: `./geoip --account-id="YOUR_ACCOUNT_ID" --edition=GeoLite2-Country --license="YOUR_LICENSE_KEY"`
   ```
   -a, --account-id int       Required: Sign up and generate this in the Maxmind website
   -l, --license string       Required: Sign up and generate this in the Maxmind website
   -b, --bind string          The address to bind to (default "0.0.0.0")
   -e, --edition string       Edition of database to download (default "GeoLite2-City")
   -p, --port string          Port to listen on (default "8080")
   -r, --route-prefix string  Route prefix for GeoIP service, cant be empty (default "/geoip")
   -u, --update-interval int  Intervals (hour) to check for database updates (default 24)
   ```
4. That's it, you can now query the api: `curl https://localhost:8080/geoip/50.19.0.1`

### Building with Docker:

1. `docker build -t geoip-server .`
1. `docker run -p 8080:8080 geoip-server /geoip -l LICENSE_TOKEN -a ACCOUNT_ID`
1. `curl localhost:8080/geoip/json/50.19.0.1`
