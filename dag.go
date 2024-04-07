package merkledag

import (
	"encoding/json"
	"hash"
)

const (
	LIST_LIMIT  = 2048
	BLOCK_LIMIT = 256 * 1024
)

const (
	BLOB = "blob"
	LIST = "list"
	TREE = "tree"
)

type Link struct {
	Name string
	Hash []byte
	Size int
}

type Object struct {
	Links []Link
	Data  []byte
}

func Add(store KVStore, node Node, h hash.Hash) []byte {
	// TODO Write the shard to KVStore and return Merkle Root

	if node.Type() == FILE {
		file := node.(File)
		fileSlice := storeFile(file, store, h)
		jsonData, _ := json.Marshal(fileSlice)
		h.Write(jsonData)
		return h.Sum(nil)
	} else {
		dir := node.(Dir)
		dirSlice := storeDirectory(dir, store, h)
		jsonData, _ := json.Marshal(dirSlice)
		h.Write(jsonData)
		return h.Sum(nil)
	}
}

func compute(height int, node File, store KVStore, seedId int, h hash.Hash) (*Object, int) {
	if height == 1 {
		if (len(node.Bytes()) - seedId) <= 256*1024 {
			data := node.Bytes()[seedId:]
			blob := Object{
				Links: nil,
				Data:  data,
			}
			jsonData, _ := json.Marshal(blob)
			h.Reset()
			h.Write(jsonData)
			exists, _ := store.Has(h.Sum(nil))
			if !exists {
				store.Put(h.Sum(nil), data)
			}
			return &blob, len(data)
		}
		links := &Object{}
		totalLen := 0
		for i := 1; i <= 4096; i++ {
			end := seedId + 256*1024
			if len(node.Bytes()) < end {
				end = len(node.Bytes())
			}
			data := node.Bytes()[seedId:end]
			blob := Object{
				Links: nil,
				Data:  data,
			}
			totalLen += len(data)
			jsonData, _ := json.Marshal(blob)
			h.Reset()
			h.Write(jsonData)
			exists, _ := store.Has(h.Sum(nil))
			if !exists {
				store.Put(h.Sum(nil), data)
			}
			links.Links = append(links.Links, Link{
				Hash: h.Sum(nil),
				Size: len(data),
			})
			links.Data = append(links.Data, []byte("data")...)
			seedId += 256 * 1024
			if seedId >= len(node.Bytes()) {
				break
			}
		}
		jsonData, _ := json.Marshal(links)
		h.Reset()
		h.Write(jsonData)
		exists, _ := store.Has(h.Sum(nil))
		if !exists {
			store.Put(h.Sum(nil), jsonData)
		}
		return links, totalLen
	} else {
		links := &Object{}
		totalLen := 0
		for i := 1; i <= 4096; i++ {
			if seedId >= len(node.Bytes()) {
				break
			}
			child, childLen := compute(height-1, node, store, seedId, h)
			totalLen += childLen
			jsonData, _ := json.Marshal(child)
			h.Reset()
			h.Write(jsonData)
			links.Links = append(links.Links, Link{
				Hash: h.Sum(nil),
				Size: childLen,
			})
			typeName := "link"
			if child.Links == nil {
				typeName = "data"
			}
			links.Data = append(links.Data, []byte(typeName)...)
		}
		jsonData, _ := json.Marshal(links)
		h.Reset()
		h.Write(jsonData)
		exists, _ := store.Has(h.Sum(nil))
		if !exists {
			store.Put(h.Sum(nil), jsonData)
		}
		return links, totalLen
	}
}

func storeFile(node File, store KVStore, h hash.Hash) *Object {
	if len(node.Bytes()) <= 256*1024 {
		data := node.Bytes()
		blob := Object{
			Links: nil,
			Data:  data,
		}
		jsonData, _ := json.Marshal(blob)
		h.Reset()
		h.Write(jsonData)
		exists, _ := store.Has(h.Sum(nil))
		if !exists {
			store.Put(h.Sum(nil), data)
		}
		return &blob
	}
	linkLen := (len(node.Bytes()) + (256*1024 - 1)) / (256 * 1024)
	height := 0
	tmp := linkLen
	for {
		height++
		tmp /= 4096
		if tmp == 0 {
			break
		}
	}
	res, _ := compute(height, node, store, 0, h)
	return res
}

func storeDirectory(node Dir, store KVStore, h hash.Hash) *Object {
	iter := node.It()
	tree := &Object{}
	for iter.Next() {
		elem := iter.Node()
		if elem.Type() == FILE {
			file := elem.(File)
			fileSlice := storeFile(file, store, h)
			jsonData, _ := json.Marshal(fileSlice)
			h.Reset()
			h.Write(jsonData)
			tree.Links = append(tree.Links, Link{
				Hash: h.Sum(nil),
				Size: int(file.Size()),
				Name: file.Name(),
			})
			elemType := "link"
			if fileSlice.Links == nil {
				elemType = "data"
			}
			tree.Data = append(tree.Data, []byte(elemType)...)
		} else {
			dir := elem.(Dir)
			dirSlice := storeDirectory(dir, store, h)
			jsonData, _ := json.Marshal(dirSlice)
			h.Reset()
			h.Write(jsonData)
			tree.Links = append(tree.Links, Link{
				Hash: h.Sum(nil),
				Size: int(dir.Size()),
				Name: dir.Name(),
			})
			elemType := "tree"
			tree.Data = append(tree.Data, []byte(elemType)...)
		}
	}
	jsonData, _ := json.Marshal(tree)
	h.Reset()
	h.Write(jsonData)
	exists, _ := store.Has(h.Sum(nil))
	if !exists {
		store.Put(h.Sum(nil), jsonData)
	}
	return tree
}