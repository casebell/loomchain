package store

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/iavl"
	"github.com/tendermint/tendermint/libs/db"

	"github.com/loomnetwork/loomchain/log"
)

const (
	numBlocks = 20
	blockSize = 20
)

var (
	blocks []*iavl.Program
	tree   *iavl.MutableTree
)

// maxVersions can be used to specify how many versions should be retained, if set to zero then
// old versions will never been deleted.
// targetVersion can be used to load any previously saved version of the store, if set to zero then
// the last version that was saved will be loaded.
// saveFrequency says how often the IVAL tree will be saved to the disk. 0 means every block.
// versionFrequency = N, indicates that versions other than multiples of N will be eventually pruned providing maxVersions >0.
func TestDualIavlStore(t *testing.T) {
	log.Setup("debug", "file://-")
	log.Root.With("module", "dual-iavlstore")

	blocks = iavl.GenerateBlocks(numBlocks, blockSize)
	tree = iavl.NewMutableTree(db.NewMemDB(), 0)
	for _, program := range blocks {
		require.NoError(t, program.Execute(tree))
	}

	t.Run("normal", testNormal)
	t.Run("max versions & max versions", testMaxVersionFrequency)
	t.Run("max versions", testMaxVersions)
	t.Run("save frequency", testSaveFrequency)
	t.Run("max versions, max versions & save frequency", testMaxVersionFrequencySaveFrequency)
}

func testNormal(t *testing.T) {
	diskDb := db.NewMemDB()
	store, err := NewDelayIavlStore(diskDb, 0, 0, 0, 0)
	require.NoError(t, err)
	executeBlocks(t, blocks, *store)

	diskTree := iavl.NewMutableTree(diskDb, 0)
	_, err = diskTree.Load()
	require.NoError(t, err)
	for _, entry := range store.Range(nil) {
		_, value := tree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, entry.Value))
		_, diskValue := diskTree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, diskValue))
	}
	tree.Iterate(func(key []byte, value []byte) bool {
		require.Zero(t, bytes.Compare(value, store.Get(key)))
		_, diskValue := diskTree.Get(key)
		require.Zero(t, bytes.Compare(value, diskValue))
		return true
	})
}

func testMaxVersions(t *testing.T) {
	const maxVersions = 5

	diskDb := db.NewMemDB()
	store, err := NewDelayIavlStore(diskDb, maxVersions, 0, 0, 0)
	require.NoError(t, err)
	executeBlocks(t, blocks, *store)

	for i := 1; i <= numBlocks; i++ {
		require.Equal(t,
			i > numBlocks-maxVersions,
			store.tree.VersionExists(int64(i)),
		)
	}
	diskTree := iavl.NewMutableTree(diskDb, 0)
	_, err = diskTree.Load()
	require.NoError(t, err)
	for _, entry := range store.Range(nil) {
		_, value := tree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, entry.Value))
		_, diskValue := diskTree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, diskValue))
	}
	tree.Iterate(func(key []byte, value []byte) bool {
		require.Zero(t, bytes.Compare(value, store.Get(key)))
		_, diskValue := diskTree.Get(key)
		require.Zero(t, bytes.Compare(value, diskValue))
		return true
	})
}

func testMaxVersionFrequency(t *testing.T) {
	const versionFrequency = 6
	const maxVersions = 5

	diskDb := db.NewMemDB()
	store, err := NewDelayIavlStore(diskDb, maxVersions, 0, 0, versionFrequency)
	require.NoError(t, err)
	executeBlocks(t, blocks, *store)

	for i := 1; i <= numBlocks; i++ {
		require.Equal(t,
			i > numBlocks-maxVersions || i%versionFrequency == 0,
			store.tree.VersionExists(int64(i)),
		)
	}
	diskTree := iavl.NewMutableTree(diskDb, 0)
	_, err = diskTree.Load()
	require.NoError(t, err)
	for _, entry := range store.Range(nil) {
		_, value := tree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, entry.Value))
		_, diskValue := diskTree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, diskValue))
	}
	tree.Iterate(func(key []byte, value []byte) bool {
		require.Zero(t, bytes.Compare(value, store.Get(key)))
		_, diskValue := diskTree.Get(key)
		require.Zero(t, bytes.Compare(value, diskValue))
		return true
	})
}

func testSaveFrequency(t *testing.T) {
	const saveFrequency = 7

	diskDb := db.NewMemDB()
	store, err := NewDelayIavlStore(diskDb, 0, 0, saveFrequency, 0)
	require.NoError(t, err)
	executeBlocks(t, blocks, *store)

	diskTree := iavl.NewMutableTree(diskDb, 0)
	_, err = diskTree.Load()
	require.NoError(t, err)
	for i := 1; i <= numBlocks; i++ {
		require.Equal(t,
			i/saveFrequency < numBlocks/saveFrequency || i%saveFrequency == 0,
			diskTree.VersionExists(int64(i)),
		)
	}
	for _, entry := range store.Range(nil) {
		_, value := tree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, entry.Value))
	}
	tree.Iterate(func(key []byte, value []byte) bool {
		require.Zero(t, bytes.Compare(value, store.Get(key)))
		return true
	})
}

func testMaxVersionFrequencySaveFrequency(t *testing.T) {
	const saveFrequency = 7
	const versionFrequency = 5
	const maxVersions = 3

	diskDb := db.NewMemDB()
	store, err := NewDelayIavlStore(diskDb, maxVersions, 0, saveFrequency, versionFrequency)
	require.NoError(t, err)
	executeBlocks(t, blocks, *store)

	diskTree := iavl.NewMutableTree(diskDb, 0)
	_, err = diskTree.Load()
	require.NoError(t, err)
	for i := 1; i <= numBlocks; i++ {
		lastSave := (numBlocks / saveFrequency) * saveFrequency
		if i <= lastSave {
			require.Equal(t,
				(i%versionFrequency == 0 || i > lastSave-maxVersions-1),
				diskTree.VersionExists(int64(i)),
			)
		} else {
			require.False(t, diskTree.VersionExists(int64(i)))
		}
	}
	for _, entry := range store.Range(nil) {
		_, value := tree.Get(entry.Key)
		require.Zero(t, bytes.Compare(value, entry.Value))
	}
	tree.Iterate(func(key []byte, value []byte) bool {
		require.Zero(t, bytes.Compare(value, store.Get(key)))
		return true
	})
}

func executeBlocks(t *testing.T, blocks []*iavl.Program, store IAVLStore) {
	for _, block := range blocks {
		require.NoError(t, block.Execute(store.tree))
		_, _, err := store.SaveVersion()
		require.NoError(t, err)
		require.NoError(t, store.Prune())
	}
}
