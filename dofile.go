package redux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gyepisam/fileutils"
)

// A DoInfo represents an active do file.
type DoInfo struct {
	Dir     string
	Name    string
	RelDir  string   //relative directory to target from do script.
	Missing []string //more specific do scripts that were not found.
}

func (do *DoInfo) Path() string {
	return filepath.Join(do.Dir, do.Name)
}

func (do *DoInfo) RelPath(path string) string {
	return filepath.Join(do.RelDir, path)
}

/*
findDofile searches for the most specific .do file for the target and, if found, returns a DoInfo
structure whose Missing field is an array of paths to more specific .do files, if any, that were not found.

Multiple extensions do not change the $2 argument to the .do script, which still only has one level of
extension removed.
*/
func (f *File) findDoFile() (*DoInfo, error) {

	candidates := []string{f.Name + ".do"}
	ext := strings.Split(f.Name, ".")
	for i := 0; i < len(ext); i++ {
		candidates = append(candidates, strings.Join(append(append([]string{"default"}, ext[i+1:]...), "do"), "."))
	}

	relPath := &RelPath{}
	var missing []string

	dir := f.Dir

TOP:
	for {

		for _, candidate := range candidates {
			path := filepath.Join(dir, candidate)
			exists, err := fileutils.FileExists(path)
			if err != nil {
				return nil, err
			} else if exists {
				return &DoInfo{dir, candidate, relPath.Join(), missing}, nil
			} else {
				missing = append(missing, path)
			}
		}

		if dir == f.RootDir {
			break TOP
		}
		relPath.Add(filepath.Base(dir))
		dir = filepath.Dir(dir)
	}

	return &DoInfo{Missing: missing}, nil
}

const shell = "/bin/sh"

// RunDoFile executes the do file script, records the metadata for the resulting output, then
// saves the resulting output to the target file, if applicable.
func (target *File) RunDoFile(doInfo *DoInfo) (err error) {
	/*

			  The execution is equivalent to:

			  sh target.ext.do target.ext target tmp0 > tmp1

			  A well behaved .do file writes to stdout (tmp0) or to the $3 file (tmp1), but not both.

			  We use two temp files so as to detect when the .do script misbehaves,
		      in order to avoid producing incorrect output.
	*/

	targetPath := target.Fullpath()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	// If the do file is a task, stdout is not redirected
	out := os.Stdout
	cleanOut := true
	outPath := targetPath + ".out.tmp"
	if !target.IsTask() {
		out, err = os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()
		defer func() {
			if cleanOut {
				_ = os.Remove(outPath)
			}
		}()
	}

	dstPath := targetPath + ".dst.tmp"
	cleanDst := true
	defer func() {
		if cleanDst {
			_ = os.Remove(dstPath)
		}
	}()

	err = target.runCmd(out, dstPath, doInfo)
	if err != nil {
		return err
	}

	if target.IsTask() {
		// Task files should not write to the temp file.
		fi, err := os.Stat(dstPath)
		switch {
		case err != nil && !os.IsNotExist(err):
			return err

		case err == nil && fi.Size() > 0:
			return target.Errorf("Task do file %s unexpectedly wrote to $3", target.DoFile)
		}

		return nil
	}

	dstExists := true
	if _, err := os.Stat(dstPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		dstExists = false
	}

	fi, err := out.Stat()
	if err != nil {
		return err
	}
	outHasData := fi.Size() > 0

	// Pick what output to preserve
	switch {
	case !dstExists && outHasData:
		// use out
		if err := os.Rename(outPath, targetPath); err != nil {
			return err
		}
		cleanOut = false

	case !dstExists && !outHasData:
		// do not create target; ensure previous target is removed
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return err
		}

	case dstExists && !outHasData:
		// use dst
		if err := os.Rename(dstPath, targetPath); err != nil {
			return err
		}
		cleanDst = false

	case dstExists && outHasData:
		return target.Errorf(".do file %s wrote to stdout and to file $3", target.DoFile)
	}

	return nil
}

func (target *File) runCmd(out *os.File, dstPath string, doInfo *DoInfo) error {

	args := []string{"-e"}

	if ShellArgs != "" {
		if ShellArgs[0] != '-' {
			ShellArgs = "-" + ShellArgs
		}
		args = append(args, ShellArgs)
	}

	relTarget := doInfo.RelPath(target.Name)
	basename := target.Name
	const (
		prefix = "default."
		suffix = ".do"
	)
	if strings.HasPrefix(doInfo.Name, prefix) && strings.HasSuffix(doInfo.Name, suffix) {
		common := doInfo.Name[len(prefix)-1:]
		common = common[:len(common)-len(suffix)]
		basename = strings.TrimSuffix(basename, common)
	}
	basename = doInfo.RelPath(basename)
	args = append(args, doInfo.Name, relTarget, basename, dstPath)

	target.Debug("@sh %s $3\n", strings.Join(args[0:len(args)-1], " "))

	cmd := exec.Command(shell, args...)
	cmd.Dir = doInfo.Dir
	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	depth := os.Getenv("REDO_DEPTH")
	parent := os.Getenv("REDO_PARENT")

	// Add environment variables, replacing existing entries if necessary.
	cmdEnv := os.Environ()
	env := map[string]string{
		"REDO_PARENT":     relTarget,
		"REDO_PARENT_DIR": doInfo.Dir,
		"REDO_DEPTH":      depth + " ",
	}

	// Update environment values if they exist and append when they dont.
TOP:
	for key, value := range env {
		prefix := key + "="
		for i, entry := range cmdEnv {
			if strings.HasPrefix(entry, prefix) {
				cmdEnv[i] = prefix + value
				continue TOP
			}
		}
		cmdEnv = append(cmdEnv, prefix+value)
	}

	cmd.Env = cmdEnv

	if Verbose() {
		prefix := depth
		if parent != "" {
			prefix += parent + " => "
		}
		target.Log("%s%s (%s)\n", prefix, target.Rel(target.Fullpath()), target.Rel(doInfo.Path()))
	}

	err := cmd.Run()
	if err == nil {
		return nil
	}

	if Verbose() {
		return target.Errorf("%s %s: %s", shell, strings.Join(args, " "), err)
	}

	return target.Errorf("%s", err)
}
