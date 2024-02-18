package referral

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// obstruction pad
var charToObstructed map[rune]rune
var obstructedToChar map[rune]rune

const ENCODING_VERSION = 1

// shuffle letters deterministically
func shuffleLetters() {
	var padShuffled []rune
	pad := make([]rune, 38) // 26 letters + '_' + '-' + 0...9
	for i := 0; i < 26; i++ {
		pad[i] = rune('A' + i)
	}
	for i := 26; i < 37; i++ {
		pad[i] = rune('0' + i - 26)
	}
	pad[36] = '-'
	pad[37] = '_'
	//"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	r := rand.New(rand.NewSource(42))
	padShuffled = make([]rune, len(pad))
	copy(padShuffled, pad)
	r.Shuffle(len(pad), func(i, j int) {
		pad[i], pad[j] = pad[j], pad[i]
	})
	// fill mapping
	charToObstructed = make(map[rune]rune)
	obstructedToChar = make(map[rune]rune)
	for i := 0; i < len(pad); i++ {
		charToObstructed[pad[i]] = padShuffled[i]
		obstructedToChar[padShuffled[i]] = pad[i]
	}
}

// transform character into mapped character
func obstruct(char rune) rune {
	ch := charToObstructed[char]
	if ch == 0 {
		ch = charToObstructed['-']
	}
	return ch
}

// transform mapped character back into original character
func deObstruct(char rune) rune {
	return obstructedToChar[char]
}

// obstructCode transforms a code consisting of letters from
// A-Z and underscore and dash into an obstructed code
func obstructCode(code string) string {
	if len(charToObstructed) == 0 {
		shuffleLetters()
	}
	obstructed := make([]rune, len(code))
	for i, char := range code {
		obstructed[i] = obstruct(char)
	}
	return string(obstructed)
}

// deObstructCode decodes a a code obstructed via obstructCode
func deObstructCode(oc string) string {
	if len(charToObstructed) == 0 {
		shuffleLetters()
	}
	deObstructed := make([]rune, len(oc))
	for i, char := range oc {
		deObstructed[i] = deObstruct(char)
	}
	return string(deObstructed)
}

// obstructMsg obstructs the onchain message of the form
// batchTs.<code>.<poolId>.<encodingversion>
func obstructMsg(msg string) string {
	s := strings.Split(msg, ".")
	code := obstructCode(s[1])
	return s[0] + "." + code + "." + s[2] + "." + s[3]
}

// deObstructMsg obstructs the onchain message of the form
// batchTs.<code>.<poolId>.<encodingversion>
func deObstructMsg(msg string) string {
	s := strings.Split(msg, ".")
	code := deObstructCode(s[1])
	return s[0] + "." + code + "." + s[2] + "." + s[3]
}

func EncodePaymentInfo(batchTs string, code string, poolId int) string {
	msg := batchTs + "." + code + "." + strconv.Itoa(poolId) + "." + strconv.Itoa(ENCODING_VERSION)
	return obstructMsg(msg)
}

func decodePaymentInfo(msg string) []string {
	if isV0Pattern(msg) {
		// format is batchTs.<code>.<poolId> with unobstructed code
		return strings.Split(msg, ".")
	}
	if isV1Pattern(msg) {
		// format is batchTs.<code>.<poolId>.<encodingversion> with unobstructed code
		msg = deObstructMsg(msg)
		return strings.Split(msg, ".")
	}
	return nil
}

func isMsgVersionCurrent(msg string) bool {
	s := strings.Split(msg, ".")
	if len(s) != 4 {
		return false
	}
	v, err := strconv.Atoi(s[3])
	if err != nil {
		return false
	}
	return v == ENCODING_VERSION
}

func isV1Pattern(msg string) bool {
	//batchTs.<code>.<poolId>.<encodingversion>
	pattern := `^\d+\.[A-Z0-9_-]+\.\d+\.\d+$`
	regex := regexp.MustCompile(pattern)
	return regex.MatchString(msg) && isMsgVersionCurrent(msg)
}

func isV0Pattern(msg string) bool {
	//batchTs.<code>.<poolId>
	pattern := `^\d+\.[A-Z0-9_-]+\.\d+$`
	regex := regexp.MustCompile(pattern)
	return regex.MatchString(msg)
}
