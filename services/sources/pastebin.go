package sources

import (
	"time"
	"fmt"
	"encoding/json"

	"github.com/OWASP/Amass/services"
	"github.com/OWASP/Amass/config"
	eb "github.com/OWASP/Amass/eventbus"
	"github.com/OWASP/Amass/resolvers"
	"github.com/OWASP/Amass/requests"
	"github.com/OWASP/Amass/net/http"
)

type DataItem struct {
    Id   string          `json:"id"`
    Tags string          `json:"tags"`
    Time string 	     `json:"time"` 
}

// Extract the response from the Web response given by pastebin
type Data struct {
	Search   	string       `json:"search"`
	Count   int          `json:"count"`
	Data   	[]DataItem   `json:"data"`
}


// Pastebin is the Service that handles access to the CertSpotter data source.
type Pastebin struct {
	services.BaseService

	SourceType string
	RateLimit  time.Duration
}

// NewPastebin returns he object initialized, but not yet started.
func NewPastebin(cfg *config.Config, bus *eb.EventBus, pool *resolvers.ResolverPool) *Pastebin {
	p := &Pastebin{
		SourceType: requests.API,
		RateLimit:  3 * time.Second,
	}

	p.BaseService = *services.NewBaseService(p, "Pastebin", cfg, bus, pool)
	return p
}


// OnStart implements the Service interface
func (p *Pastebin) OnStart() error {
	p.BaseService.OnStart()


	go p.processRequests()
	return nil
}

func (p *Pastebin) processRequests() {
	last := time.Now()

	for {
		select {
		case <-p.Quit():
			return
		case req := <-p.DNSRequestChan():
			if p.Config().IsDomainInScope(req.Domain) {
				if time.Now().Sub(last) < p.RateLimit {
					time.Sleep(p.RateLimit)
				}
				last = time.Now()
				p.executeQuery(req.Domain)
				last = time.Now()
			}
		case <-p.AddrRequestChan():
		case <-p.ASNRequestChan():
		case <-p.WhoisRequestChan():
		}
	}
}

func (p *Pastebin) executeQuery(domain string) {
	var url, page string
	var err error
	re := p.Config().DomainRegex(domain)

	ids, err := p.extractIDs(domain,url)
	if err != nil {
		p.Bus().Publish(requests.LogTopic, fmt.Sprintf("%s: %s: %v", p.String(), url, err))
		return
	}

	for _, id := range ids {
		url = p.webURLDumpData(id)
		page, err = http.RequestWebPage(url, nil, nil, "", "")
		if err != nil {
			p.Bus().Publish(requests.LogTopic, fmt.Sprintf("%s: %s: %v", p.String(), url, err))
			return 
		}
		for _, name := range re.FindAllString(page, -1) {
			p.Bus().Publish(requests.NewNameTopic, &requests.DNSRequest{
				Name:   name,
				Domain: domain,
				Tag:    p.SourceType,
				Source: p.String(),
			})
		}
	}
}

// Extract the IDs from the pastebin Web response
func (p *Pastebin) extractIDs(domain string, url string) ([]string,error) {
	var page string
	var data Data
	var err error
	var ids []string

	url = p.webURLDumpIDs(domain)
	page, err = http.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		p.Bus().Publish(requests.LogTopic, fmt.Sprintf("%s: %s: %v", p.String(), url, err))
		return nil, err
	}

	err = json.Unmarshal([]byte(page), &data)
    if err != nil {
        panic(err)
	}
	
	for _, item := range data.Data {
		ids = append(ids,item.Id)
	} 
	
	return ids, nil
}


// Returns the Web URL to fetch all dump ids for a given doamin
func (p *Pastebin) webURLDumpIDs(domain string) string {
	return fmt.Sprintf("https://psbdmp.ws/api/search/%s", domain)
}

// Returns the Web URL to get all dumps for a given doamin
func (p *Pastebin) webURLDumpData(id string) string {
	return fmt.Sprintf("https://psbdmp.ws/api/dump/get/%s",id)
}