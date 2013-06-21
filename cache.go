package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

func (db DB) CacheNode(node *Node, expiry int) (err error) {
	stmt, err := db.Prepare(`INSERT INTO nodes_cached
(address, owner, lat, lon, status, expiration)
VALUES(?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return
	}
	_, err = stmt.Exec(node.Addr, node.OwnerName,
		node.Latitude, node.Longitude, node.SourceID, node.Status)
	stmt.Close()
	return
}

func (db DB) CacheNodes(nodes []*Node) (err error) {
	stmt, err := db.Prepare(`INSERT INTO nodes_cached
(address, owner, lat, lon, status, source, retrieved)
VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return
	}

	for _, node := range nodes {
		retrieved := node.RetrieveTime
		if retrieved == 0 {
			retrieved = time.Now().Unix()
		}
		_, err = stmt.Exec([]byte(node.Addr), node.OwnerName,
			node.Latitude, node.Longitude,
			node.Status, node.SourceID, retrieved)
		if err != nil {
			return
		}
	}
	stmt.Close()
	return
}

// GetMapSourceToID returns a mapping of child map hostnames to their
// local IDs. It also includes a mapping of "local" to id 0.
func (db DB) GetMapSourceToID() (sourceToID map[string]int, err error) {
	// Initialize the map and insert the "local" id.
	sourceToID = make(map[string]int, 1)
	sourceToID["local"] = 0

	// Retrieve every pair of hostnames and IDs.
	rows, err := db.Query(`SELECT hostname,id
FROM cached_maps;`)
	if err == sql.ErrNoRows {
		return sourceToID, nil
	} else if err != nil {
		return
	}

	// Put in the rest of the mappings.
	for rows.Next() {
		var hostname string
		var id int
		if err = rows.Scan(&hostname, &id); err != nil {
			return
		}
		sourceToID[hostname] = id
	}

	return
}

// GetMapIDToSource returns a mapping of local IDs to public
// hostnames. ID 0 is "local".
func (db DB) GetMapIDToSource() (IDToSource map[int]string, err error) {
	// Initialize the slice with "local".
	IDToSource = make(map[int]string, 1)
	IDToSource[0] = "local"

	// Retrieve every pair of IDs and hostnames.
	rows, err := db.Query(`SELECT id,hostname
FROM cached_maps;`)
	if err == sql.ErrNoRows {
		return IDToSource, nil
	} else if err != nil {
		return
	}

	// Put in the rest of the IDs.
	for rows.Next() {
		var id int
		var hostname string
		if err = rows.Scan(&id, &hostname); err != nil {
			return
		}
		IDToSource[id] = hostname
	}
	return
}

func (db DB) FindSourceMap(id int) (source string, err error) {
	if id == 0 {
		return "local", nil
	}
	row := db.QueryRow(`SELECT hostname
FROM cached_maps
WHERE id=?`, id)

	err = row.Scan(&source)
	return
}

func (db DB) CacheFormatNodes(nodes []*Node) (sourceMaps map[string][]*Node, err error) {
	// First, get a mapping of IDs to sources for quick access.
	idSources, err := db.GetMapIDToSource()
	if err != nil {
		return
	}

	// Now, prepare the data to be returned. Nodes will be added one
	// at a time to the key arrays.
	sourceMaps = make(map[string][]*Node)
	for _, node := range nodes {
		hostname := idSources[node.SourceID]
		sourcemapNodes := sourceMaps[hostname]
		if sourcemapNodes == nil {
			sourcemapNodes = make([]*Node, 0, 5)
		}

		sourceMaps[hostname] = append(sourcemapNodes, node)
	}
	return
}

// nodeDumpWrapper is a structure which wraps a response from /api/all
// in which the Data field is a map[string][]*Node.
type nodeDumpWrapper struct {
	Data  map[string][]*Node `json:"data"`
	Error interface{}        `json:"error"`
}

// GetAllFromChildMap retrieves a list of nodes from a single remote
// address, and localizes them. If it encounters a remote address that
// is not already known, it safely adds it to the sourceToID map. It
// is safe for concurrent use. If it encounters an error, it will log
// it and return nil.
func GetAllFromChildMap(address string, sourceToID *map[string]int,
	sourceMutex *sync.RWMutex) (nodes []*Node) {
	// Try to get all nodes via the API.
	resp, err := http.Get("http://" +
		strings.TrimRight(address, "/") + "/api/all")
	if err != nil {
		l.Errf("Caching %q produced: %s", address, err)
		return nil
	}

	// Read the data into a the nodeDumpWrapper type, so that it
	// decodes properly.
	var jresp nodeDumpWrapper
	err = json.NewDecoder(resp.Body).Decode(&jresp)
	if err != nil {
		l.Errf("Caching %q produced: %s", address, err)
		return nil
	} else if jresp.Error != nil {
		l.Errf("Caching %q produced remote error: %s",
			address, jresp.Error)
		return nil
	}

	// Prepare an initial slice so that it can be appended to, then
	// loop through and convert sources to IDs.
	//
	// Additionally, use a boolean to keep track of whether we've
	// replaced "local" with the actual address already, to save some
	// needless compares.
	nodes = make([]*Node, 0)
	var replacedLocal bool
	for source, remoteNodes := range jresp.Data {
		// If we come across "local", then replace it with the address
		// we're retrieving from.
		if !replacedLocal && source == "local" {
			source = address
		}

		// First, check if the source is known. If not, then we need
		// to add it and refresh our map. Make sure all reads and
		// writes to sourceToID are threadsafe.
		sourceMutex.RLock()
		id, ok := (*sourceToID)[source]
		sourceMutex.RUnlock()
		if !ok {
			// Add the new ID as the len(sourceToID), because that
			// should be unique, under our ID scheme.
			sourceMutex.Lock()
			id = len(*sourceToID) + 1
			(*sourceToID)[source] = id
			sourceMutex.Unlock()

			l.Debugf("Discoverd new source map %q, ID %d\n",
				source, id)
		}

		// Once the ID is set, proceed on to add it in all the
		// remoteNodes.
		for _, n := range remoteNodes {
			n.SourceID = id
		}

		// Finally, append remoteNodes to the slice we're returning.
		nodes = append(nodes, remoteNodes...)
	}
	return
}
