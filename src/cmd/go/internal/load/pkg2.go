package load

import (
	"context"
	"encoding/json"
	"fmt"
	"go/build"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func loadPackageDataCached(ctx context.Context,
	path, parentPath, parentDir, parentRoot string,
	parentIsStd bool, mode int) (bp *build.Package, loaded bool, err error) {

	// 1. Does there exist a Cache?
	// If so, we are done.
	//
	// If not, nothing is cached so call
	// ctx.ImportDir.
	c, err := loadCache(cacheDir, modulePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to initialize build cache at %s: %s\n", cacheDir+modulePath, err)
	}
	if c.exists {
		// Do something to return a bp.
		return &build.Package{}, true, nil
	}

	// 2. The cache does not exist.
	// Load the package, then write it to the cache.
	fmt.Printf("Cache does not exist; importing %q from %q\n\n", path, srcDir)
	bp, loaded, err = loadPackageData(ctx, path, parentPath, srcDir, parentRoot, parentIsStd, mode)
	if err != nil {
		return nil, false, err
	}

	// We have the:
	// - Package
	// - Loaded all the files of that package
	goFiles := StringList(
		pkg.GoFiles,
		pkg.TestGoFiles,
		pkg.XTestGoFiles,
	)

	fmt.Println("Looping through all of the files in the package...")

	if c.Dirs == nil {
		c.Dirs = map[string]*Dir{}
	}
	d := &Dir{
		Path:          pkg.Dir,
		Name:          pkg.Name,
		ImportComment: pkg.ImportComment,
		Doc:           pkg.Doc,
		ImportPath:    pkg.ImportPath,
		Root:          pkg.Root,
		SrcRoot:       pkg.SrcRoot,
		PkgRoot:       pkg.PkgRoot,
	}
	c.Dirs[pkg.Dir] = d
	for _, file := range goFiles {
		fset := token.NewFileSet()
		info := &build.FileInfo{Fset: fset, Name: filepath.Join(pkg.Dir, file)}
		f, err := os.Open(filepath.Join(pkg.Dir, file))
		if err != nil {
			return nil, false, err
		}
		if err := build.ReadGoInfo(f, info); err != nil {
			return nil, false, err
		}
		f.Close()

		fi := &FileInfo{Name: info.Name}
		for _, imp := range info.Imports {
			fi.Imports = append(fi.Imports, FileImport{Path: imp.Path, Pos: imp.Pos})
		}
		for _, emb := range info.Embeds {
			fi.Embeds = append(fi.Embeds, FileEmbed{Pattern: emb.Pattern, Pos: emb.Pos})
		}

		d.GoFiles = append(d.GoFiles, fi)
		syms, err := loadIdentifiers(pkg.ImportPath, filepath.Join(pkg.Dir, file))
		if err != nil {
			return nil, false, err
		}
		fi.Exports = syms
		fi.BuildTags, err = builds(info.Header)
		if err != nil {
			return nil, false, err
		}
	}

	nonGoFiles := StringList(
		pkg.CgoFiles,
		pkg.CFiles,
		pkg.CXXFiles,
		pkg.MFiles,
		pkg.HFiles,
		pkg.FFiles,
		pkg.SFiles,
		pkg.SwigFiles,
		pkg.SwigCXXFiles,
		pkg.SysoFiles,
	)
	for _, file := range nonGoFiles {
		d.NonGoFiles = append(d.NonGoFiles, file)
	}

	fmt.Println("Marshaling data to file")
	data, err := json.MarshalIndent(&c.Dirs, "", "\t")
	if err == nil {
		data = append(data, '\n')
		// Write to the cache
		fmt.Println("Writing to cache...")
		c.PutBytes(data)
	}
	return &build.Package{}, true, nil
}

func loadCache(cacheDir, modulePath string) (*Cache, error) {
	fmt.Printf("Loading %q from cache %q\n\n", modulePath, cacheDir)

	dir := filepath.Join(cacheDir, modulePath, "@v")
	c := &Cache{
		dir: dir,
		now: time.Now,
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: dir, Err: fmt.Errorf("not a directory")}
	}
	fn := c.fileName([HashSize]byte{}, "")
	fmt.Println(fn)
	info, err = os.Stat(fn)
	if os.IsNotExist(err) {
		return c, nil
	}

	c.exists = true
	return c, nil
}
