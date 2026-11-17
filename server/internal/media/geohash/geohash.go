package geohash

import "math"

const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

// Encode converts WGS-84 latitude/longitude to a geohash string.
func Encode(latitude, longitude float64, precision int) (string, bool) {
	if math.IsNaN(latitude) ||
		math.IsInf(latitude, 0) ||
		math.IsNaN(longitude) ||
		math.IsInf(longitude, 0) ||
		latitude < -90 ||
		latitude > 90 ||
		longitude < -180 ||
		longitude > 180 ||
		precision <= 0 {
		return "", false
	}

	latMin, latMax := -90.0, 90.0
	lonMin, lonMax := -180.0, 180.0
	hash := make([]byte, 0, precision)
	bit := 0
	value := 0
	evenBit := true

	for len(hash) < precision {
		if evenBit {
			mid := (lonMin + lonMax) / 2
			if longitude >= mid {
				value = value*2 + 1
				lonMin = mid
			} else {
				value *= 2
				lonMax = mid
			}
		} else {
			mid := (latMin + latMax) / 2
			if latitude >= mid {
				value = value*2 + 1
				latMin = mid
			} else {
				value *= 2
				latMax = mid
			}
		}

		evenBit = !evenBit
		bit++

		if bit == 5 {
			hash = append(hash, base32[value])
			bit = 0
			value = 0
		}
	}

	return string(hash), true
}
