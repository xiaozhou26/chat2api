package chatgpt_backend

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/sha3"
)

const (
	defaultPowScript = "https://chatgpt.com/backend-api/sentinel/sdk.js"
	maxAttempts      = 500000
)

var (
	cores        = []int{8, 16, 24, 32}
	screenValues = []int{3000, 4000, 5000}
	documentKeys = []string{
		"_reactListeningo743lnnpvdg",
		"location",
	}
	navigatorKeys = []string{
		"registerProtocolHandler−function registerProtocolHandler() { [native code] }",
		"storage−[object StorageManager]",
		"locks−[object LockManager]",
		"appCodeName−Mozilla",
		"permissions−[object Permissions]",
		"share−function share() { [native code] }",
		"webdriver−false",
		"managed−[object NavigatorManagedData]",
		"canShare−function canShare() { [native code] }",
		"vendor−Google Inc.",
		"mediaDevices−[object MediaDevices]",
		"vibrate−function vibrate() { [native code] }",
		"storageBuckets−[object StorageBucketManager]",
		"mediaCapabilities−[object MediaCapabilities]",
		"cookieEnabled−true",
		"virtualKeyboard−[object VirtualKeyboard]",
		"product−Gecko",
		"presentation−[object Presentation]",
		"onLine−true",
		"mimeTypes−[object MimeTypeArray]",
		"credentials−[object CredentialsContainer]",
		"serviceWorker−[object ServiceWorkerContainer]",
		"keyboard−[object Keyboard]",
		"gpu−[object GPU]",
		"doNotTrack",
		"serial−[object Serial]",
		"pdfViewerEnabled−true",
		"language−zh-CN",
		"geolocation−[object Geolocation]",
		"userAgentData−[object NavigatorUAData]",
		"getUserMedia−function getUserMedia() { [native code] }",
		"sendBeacon−function sendBeacon() { [native code] }",
		"hardwareConcurrency−32",
		"windowControlsOverlay−[object WindowControlsOverlay]",
	}
	windowKeys = []string{
		"0",
		"window",
		"self",
		"document",
		"name",
		"location",
		"customElements",
		"history",
		"navigation",
		"innerWidth",
		"innerHeight",
		"scrollX",
		"scrollY",
		"visualViewport",
		"screenX",
		"screenY",
		"outerWidth",
		"outerHeight",
		"devicePixelRatio",
		"screen",
		"chrome",
		"navigator",
		"onresize",
		"performance",
		"crypto",
		"indexedDB",
		"sessionStorage",
		"localStorage",
		"scheduler",
		"alert",
		"atob",
		"btoa",
		"fetch",
		"matchMedia",
		"postMessage",
		"queueMicrotask",
		"requestAnimationFrame",
		"setInterval",
		"setTimeout",
		"caches",
		"__NEXT_DATA__",
		"__BUILD_MANIFEST",
		"__NEXT_PRELOADREADY",
	}
	scriptSrcRE = regexp.MustCompile(`<script\b[^>]*\bsrc=["']([^"']+)["']`)
	dataBuildRE = regexp.MustCompile(`(?:c/[^/]*/_|<html[^>]*data-build=["']([^"']*)["'])`)
)

type ProofWork struct {
	Difficulty string `json:"difficulty,omitempty"`
	Required   bool   `json:"required"`
	Seed       string `json:"seed,omitempty"`
	Ospt       string `json:"-"`
}

type Resources struct {
	ScriptSources []string
	DataBuild     string
}

func ParseResources(html string) Resources {
	resources := Resources{}
	for _, match := range scriptSrcRE.FindAllStringSubmatch(html, -1) {
		resources.ScriptSources = append(resources.ScriptSources, match[1])
		if resources.DataBuild == "" {
			if build := regexp.MustCompile(`c/[^/]*/_`).FindString(match[1]); build != "" {
				resources.DataBuild = build
			}
		}
	}
	if len(resources.ScriptSources) == 0 {
		resources.ScriptSources = []string{defaultPowScript}
	}
	if resources.DataBuild == "" {
		for _, match := range dataBuildRE.FindAllStringSubmatch(html, -1) {
			if len(match) > 1 && match[1] != "" {
				resources.DataBuild = match[1]
				break
			}
			if match[0] != "" && strings.HasPrefix(match[0], "c/") {
				resources.DataBuild = match[0]
				break
			}
		}
	}
	return resources
}

func CalcProofToken(seed string, difficulty string, userAgent string, resources ...Resources) string {
	answer, ok := generate(seed, difficulty, buildConfig(userAgent, firstResource(resources)))
	if !ok {
		return ""
	}
	return "gAAAAAB" + answer
}

func LegacyRequirementsToken(userAgent string, resources ...Resources) string {
	seed := fmt.Sprintf("%v", rand.New(rand.NewSource(time.Now().UnixNano())).Float64())
	answer, _ := generate(seed, "0fffff", buildConfig(userAgent, firstResource(resources)))
	return "gAAAAAC" + answer
}

func firstResource(resources []Resources) Resources {
	if len(resources) > 0 {
		return resources[0]
	}
	return Resources{ScriptSources: []string{defaultPowScript}}
}

func buildConfig(userAgent string, resources Resources) []interface{} {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	scriptSources := resources.ScriptSources
	if len(scriptSources) == 0 {
		scriptSources = []string{defaultPowScript}
	}
	now := time.Now()
	perfMs := float64(now.UnixNano()%int64(time.Second)) / float64(time.Millisecond)
	return []interface{}{
		screenValues[rng.Intn(len(screenValues))],
		legacyParseTime(),
		int64(4294705152),
		0,
		userAgent,
		scriptSources[rng.Intn(len(scriptSources))],
		resources.DataBuild,
		"en-US",
		"en-US,es-US,en,es",
		0,
		navigatorKeys[rng.Intn(len(navigatorKeys))],
		documentKeys[rng.Intn(len(documentKeys))],
		windowKeys[rng.Intn(len(windowKeys))],
		perfMs,
		uuid.NewString(),
		"",
		cores[rng.Intn(len(cores))],
		float64(now.UnixMilli()) - perfMs,
	}
}

func legacyParseTime() string {
	loc := time.FixedZone("Eastern Standard Time", -5*60*60)
	return time.Now().In(loc).Format("Mon Jan 02 2006 15:04:05") + " GMT-0500 (Eastern Standard Time)"
}

func generate(seed string, difficulty string, config []interface{}) (string, bool) {
	target, err := hex.DecodeString(difficulty)
	if err != nil || len(target) == 0 {
		return "", false
	}
	diffLen := len(difficulty) / 2
	seedBytes := []byte(seed)
	static1 := mustJSONPrefix(config[:3])
	static2 := mustJSONMiddle(config[4:9])
	static3 := mustJSONSuffix(config[10:])
	hasher := sha3.New512()
	for i := 0; i < maxAttempts; i++ {
		finalJSON := bytes.NewBuffer(make([]byte, 0, 512))
		finalJSON.Write(static1)
		finalJSON.WriteString(fmt.Sprintf("%d", i))
		finalJSON.Write(static2)
		finalJSON.WriteString(fmt.Sprintf("%d", i>>1))
		finalJSON.Write(static3)
		encoded := []byte(base64.StdEncoding.EncodeToString(finalJSON.Bytes()))
		hasher.Write(seedBytes)
		hasher.Write(encoded)
		digest := hasher.Sum(nil)
		hasher.Reset()
		if bytes.Compare(digest[:diffLen], target) <= 0 {
			return string(encoded), true
		}
	}
	fallback := "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D" + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%q", seed)))
	return fallback, false
}

func mustJSONPrefix(values []interface{}) []byte {
	b, _ := json.Marshal(values)
	return append(b[:len(b)-1], ',')
}

func mustJSONMiddle(values []interface{}) []byte {
	b, _ := json.Marshal(values)
	return append(append([]byte{','}, b[1:len(b)-1]...), ',')
}

func mustJSONSuffix(values []interface{}) []byte {
	b, _ := json.Marshal(values)
	return append([]byte{','}, b[1:]...)
}
