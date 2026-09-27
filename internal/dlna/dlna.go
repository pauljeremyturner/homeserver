// Package dlna implements a minimal UPnP AV control point client: enough to
// poll a MediaRenderer's AVTransport service for transport state and
// now-playing metadata.
package dlna

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"time"
)

// Position holds the metadata and playback position for whatever is
// currently loaded on a renderer's AVTransport, parsed from GetPositionInfo.
type Position struct {
	Title       string
	Artist      string
	Album       string
	TrackNumber string
	AlbumArtURL string
	TrackURI    string // where the renderer is fetching the track from
	Duration    string
	RelTime     string
}

// httpClient is shared across all SOAP calls, with keep-alives disabled.
// Some cheap embedded renderers' HTTP servers advertise persistent
// connections but silently close them right after responding — Go's default
// transport would then try to reuse that dead pooled connection on the next
// poll and fail with a spurious "EOF". A fresh TCP connection per request
// avoids that at the cost of a little extra connection setup, which is a
// non-issue at this poll cadence (a few requests every few seconds).
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		DisableKeepAlives: true,
	},
}

// soapCall posts a UPnP SOAP action to a control URL and returns the raw response body.
func soapCall(controlURL, serviceType, action, args string) ([]byte, error) {
	body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:%s xmlns:u="%s">%s</u:%s>
  </s:Body>
</s:Envelope>`, action, serviceType, args, action)

	req, err := http.NewRequest("POST", controlURL, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, serviceType, action))

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("soap %s: unexpected status %d", action, resp.StatusCode)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Go's encoding/xml matches elements by local name when the struct tag omits
// a namespace, so these envelopes work unchanged against both AVTransport:1
// and AVTransport:2 renderers (they differ only in namespace URI).

type transportInfoEnvelope struct {
	Body struct {
		GetTransportInfoResponse struct {
			CurrentTransportState string `xml:"CurrentTransportState"`
		} `xml:"GetTransportInfoResponse"`
	} `xml:"Body"`
}

type positionInfoEnvelope struct {
	Body struct {
		GetPositionInfoResponse struct {
			TrackDuration string `xml:"TrackDuration"`
			TrackMetaData string `xml:"TrackMetaData"`
			RelTime       string `xml:"RelTime"`
			TrackURI      string `xml:"TrackURI"`
		} `xml:"GetPositionInfoResponse"`
	} `xml:"Body"`
}

// didlLite mirrors the fields we care about from a DIDL-Lite <item>.
// Namespace prefixes (dc:, upnp:) are stripped by encoding/xml when the tag
// carries no explicit namespace, so plain local names are enough here too.
type didlLite struct {
	Item struct {
		Title       string `xml:"title"`
		Artist      string `xml:"artist"`
		Album       string `xml:"album"`
		TrackNumber string `xml:"originalTrackNumber"`
		AlbumArtURI string `xml:"albumArtURI"`
		Res         string `xml:"res"`
	} `xml:"item"`
}

// GetTransportState calls AVTransport's GetTransportInfo and returns the
// renderer's CurrentTransportState (e.g. "PLAYING", "PAUSED_PLAYBACK", "STOPPED").
func GetTransportState(controlURL, serviceType string) (string, error) {
	resp, err := soapCall(controlURL, serviceType, "GetTransportInfo", "<InstanceID>0</InstanceID>")
	if err != nil {
		return "", err
	}
	var env transportInfoEnvelope
	if err := xml.Unmarshal(resp, &env); err != nil {
		return "", fmt.Errorf("parsing GetTransportInfo response: %w", err)
	}
	return env.Body.GetTransportInfoResponse.CurrentTransportState, nil
}

// GetPositionInfo calls AVTransport's GetPositionInfo and parses the
// embedded DIDL-Lite TrackMetaData into a Position.
func GetPositionInfo(controlURL, serviceType string) (Position, error) {
	resp, err := soapCall(controlURL, serviceType, "GetPositionInfo", "<InstanceID>0</InstanceID>")
	if err != nil {
		return Position{}, err
	}
	var env positionInfoEnvelope
	if err := xml.Unmarshal(resp, &env); err != nil {
		return Position{}, fmt.Errorf("parsing GetPositionInfo response: %w", err)
	}

	pos := Position{
		Duration: env.Body.GetPositionInfoResponse.TrackDuration,
		RelTime:  env.Body.GetPositionInfoResponse.RelTime,
		TrackURI: env.Body.GetPositionInfoResponse.TrackURI,
	}

	metaXML := env.Body.GetPositionInfoResponse.TrackMetaData
	if metaXML != "" {
		var didl didlLite
		if err := xml.Unmarshal([]byte(metaXML), &didl); err == nil {
			pos.Title = didl.Item.Title
			pos.Artist = didl.Item.Artist
			pos.Album = didl.Item.Album
			pos.TrackNumber = didl.Item.TrackNumber
			pos.AlbumArtURL = didl.Item.AlbumArtURI
			if pos.TrackURI == "" {
				pos.TrackURI = didl.Item.Res
			}
		}
	}
	return pos, nil
}

// Play calls AVTransport's Play at normal speed (resumes a paused track).
func Play(controlURL, serviceType string) error {
	_, err := soapCall(controlURL, serviceType, "Play", "<InstanceID>0</InstanceID><Speed>1</Speed>")
	return err
}

// Pause calls AVTransport's Pause.
func Pause(controlURL, serviceType string) error {
	_, err := soapCall(controlURL, serviceType, "Pause", "<InstanceID>0</InstanceID>")
	return err
}

// descriptionPaths are where common UPnP devices serve their device
// description (the LOCATION they'd announce over SSDP): Wiimu/Linkplay
// speakers and Rygel, Plex, MiniDLNA, and a few other embedded stacks.
// Probing these avoids needing SSDP multicast, which doesn't cross Docker's
// bridge network.
var descriptionPaths = []string{
	"/description.xml",
	"/DeviceDescription.xml",
	"/rootDesc.xml",
	"/desc.xml",
	"/dmr.xml",
	"/dms.xml",
}

type deviceDescription struct {
	Device struct {
		FriendlyName string `xml:"friendlyName"`
	} `xml:"device"`
}

// FriendlyName finds the UPnP device serving at base ("http://host:port")
// and returns its friendlyName, e.g. "Plex Media Server: fedora".
func FriendlyName(base string) (string, error) {
	for _, p := range descriptionPaths {
		resp, err := httpClient.Get(base + p)
		if err != nil {
			return "", err
		}
		var desc deviceDescription
		err = xml.NewDecoder(resp.Body).Decode(&desc)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK && err == nil && desc.Device.FriendlyName != "" {
			return desc.Device.FriendlyName, nil
		}
	}
	return "", fmt.Errorf("no UPnP device description found at %s", base)
}
