package container

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nektos/act/pkg/common"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

type hostExecutor struct {
	input *NewContainerInput
}

func NewHostExecutor(input *NewContainerInput) Container {
	return &hostExecutor{
		input: input,
	}
}

func (e *hostExecutor) Create(capAdd []string, capDrop []string) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (e *hostExecutor) Close() common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (e *hostExecutor) Copy(destPath string, files ...*FileEntry) common.Executor {
	return func(ctx context.Context) error {
		for _, f := range files {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(destPath, f.Name)), 0777); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(destPath, f.Name), []byte(f.Body), fs.FileMode(f.Mode)); err != nil {
				return err
			}
		}
		return nil
	}
}

func (e *hostExecutor) CopyDir(destPath string, srcPath string, useGitIgnore bool) common.Executor {
	return func(ctx context.Context) error {
		return filepath.Walk(srcPath, func(file string, fi os.FileInfo, err error) error {
			if fi.Mode()&os.ModeSymlink != 0 {
				lnk, err := os.Readlink(file)
				if err != nil {
					return err
				}
				relpath, err := filepath.Rel(srcPath, file)
				if err != nil {
					return err
				}
				fdestpath := filepath.Join(destPath, relpath)
				if err := os.MkdirAll(filepath.Dir(fdestpath), 0777); err != nil {
					return err
				}
				if err := os.Symlink(lnk, fdestpath); err != nil {
					return err
				}
			} else if fi.Mode().IsRegular() {
				relpath, err := filepath.Rel(srcPath, file)
				if err != nil {
					return err
				}
				f, err := os.Open(file)
				if err != nil {
					return err
				}
				defer f.Close()
				fdestpath := filepath.Join(destPath, relpath)
				if err := os.MkdirAll(filepath.Dir(fdestpath), 0777); err != nil {
					return err
				}
				df, err := os.OpenFile(fdestpath, os.O_CREATE|os.O_WRONLY, fi.Mode())
				if err != nil {
					return err
				}
				defer df.Close()
				if _, err := io.Copy(df, f); err != nil {
					return err
				}
			}
			return nil
		})
	}
}

func fileCallbackfilecbk(srcPath string, tw *tar.Writer, file string, fi os.FileInfo, err error) error {
	if fi.Mode()&os.ModeSymlink != 0 {
		lnk, err := os.Readlink(file)
		if err != nil {
			return err
		}
		fih, err := tar.FileInfoHeader(fi, lnk)
		if err != nil {
			return err
		}
		fih.Name, err = filepath.Rel(srcPath, file)
		if err != nil {
			return err
		}
		if string(filepath.Separator) != "/" {
			fih.Name = strings.ReplaceAll(fih.Name, string(filepath.Separator), "/")
		}
		if err := tw.WriteHeader(fih); err != nil {
			return err
		}
	} else if fi.Mode().IsRegular() {
		fih, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		fih.Name, err = filepath.Rel(srcPath, file)
		if err != nil {
			return err
		}
		if string(filepath.Separator) != "/" {
			fih.Name = strings.ReplaceAll(fih.Name, string(filepath.Separator), "/")
		}
		if err := tw.WriteHeader(fih); err != nil {
			return err
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
	}
	return nil
}

func (e *hostExecutor) GetContainerArchive(ctx context.Context, srcPath string) (rc io.ReadCloser, err error) {
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	defer func() {
		if err == nil {
			err = tw.Close()
		}
	}()
	srcPath = filepath.Clean(srcPath)
	fi, err := os.Lstat(srcPath)
	if err != nil {
		return nil, err
	}
	filecbk := func(file string, fi os.FileInfo, err error) error {
		return fileCallbackfilecbk(srcPath, tw, file, fi, err)
	}
	if fi.IsDir() {
		if err := filepath.Walk(srcPath, filecbk); err != nil {
			return nil, err
		}
	} else {
		file := srcPath
		srcPath = filepath.Dir(srcPath)
		if err := filecbk(file, fi, nil); err != nil {
			return nil, err
		}
	}
	return io.NopCloser(buf), nil
}

func (e *hostExecutor) Pull(forcePull bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (e *hostExecutor) Start(attach bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

type ptyWriter struct {
	Out       io.Writer
	AutoStop  bool
	dirtyLine bool
}

func (w *ptyWriter) Write(buf []byte) (int, error) {
	if w.AutoStop && len(buf) > 0 && buf[len(buf)-1] == 4 {
		n, err := w.Out.Write(buf[:len(buf)-1])
		if err != nil {
			return n, err
		}
		if w.dirtyLine || len(buf) > 1 && buf[len(buf)-2] != '\n' {
			w.Out.Write([]byte("\n"))
			return n, io.EOF
		}
		return n, io.EOF
	}
	w.dirtyLine = strings.LastIndex(string(buf), "\n") < len(buf)-1
	return w.Out.Write(buf)
}

func lookupPathHost(cmd string, env map[string]string, writer io.Writer) (string, error) {
	oldpath, _ := os.LookupEnv("PATH")
	os.Setenv("PATH", env["PATH"])
	defer os.Setenv("PATH", oldpath)
	f, err := exec.LookPath(cmd)
	if err != nil {
		err := "Cannot find: " + fmt.Sprint(cmd) + " in PATH"
		if _, _err := writer.Write([]byte(err + "\n")); _err != nil {
			return "", errors.Wrap(_err, err)
		}
		return "", errors.New(err)
	}
	return f, nil
}

func setupPty(cmd *exec.Cmd, cmdline string) (*os.File, *os.File, error) {
	ppty, tty, err := openPty()
	if err != nil {
		return nil, nil, err
	}
	if term.IsTerminal(int(tty.Fd())) {
		_, err := term.MakeRaw(int(tty.Fd()))
		if err != nil {
			ppty.Close()
			tty.Close()
			return nil, nil, err
		}
	}
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.SysProcAttr = getSysProcAttr(cmdline, true)
	return ppty, tty, nil
}

func writeKeepAlive(ppty io.Writer) {
	c := 1
	var err error
	for c == 1 && err == nil {
		c, err = ppty.Write([]byte{4})
		<-time.After(time.Second)
	}
}

func copyPtyOutput(writer io.Writer, ppty io.Reader, finishLog context.CancelFunc) {
	defer func() {
		finishLog()
	}()
	if _, err := io.Copy(writer, ppty); err != nil {
		return
	}
}

func (e *hostExecutor) UpdateFromImageEnv(env *map[string]string) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (e *hostExecutor) exec2(ctx context.Context, command []string, cmdline string, env map[string]string, user, workdir string) error {
	envList := getEnvListFromMap(env)
	var wd string
	if workdir != "" {
		if strings.HasPrefix(workdir, "/") {
			wd = workdir
		} else {
			wd = fmt.Sprintf("%s/%s", e.input.WorkingDir, workdir)
		}
	} else {
		wd = e.input.WorkingDir
	}
	f, err := lookupPathHost(command[0], env, e.input.Stdout)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, f)
	cmd.Path = f
	cmd.Args = command
	cmd.Stdin = nil
	cmd.Stdout = e.input.Stdout
	cmd.Env = envList
	cmd.Stderr = e.input.Stderr
	cmd.Dir = wd
	cmd.SysProcAttr = getSysProcAttr(cmdline, false)
	var ppty *os.File
	var tty *os.File
	defer func() {
		if ppty != nil {
			ppty.Close()
		}
		if tty != nil {
			tty.Close()
		}
	}()
	if e.input.Tty {
		var err error
		ppty, tty, err = setupPty(cmd, cmdline)
		if err != nil {
			common.Logger(ctx).Debugf("Failed to setup Pty %v\n", err.Error())
		}
	}
	writer := &ptyWriter{Out: e.input.Stdout}
	logctx, finishLog := context.WithCancel(context.Background())
	if ppty != nil {
		go copyPtyOutput(writer, ppty, finishLog)
	} else {
		finishLog()
	}
	if ppty != nil {
		go writeKeepAlive(ppty)
	}
	err = cmd.Run()
	if err != nil {
		return err
	}
	if tty != nil {
		writer.AutoStop = true
		if _, err := tty.Write([]byte("\x04")); err != nil {
			common.Logger(ctx).Debug("Failed to write EOT")
		}
	}
	<-logctx.Done()

	if ppty != nil {
		ppty.Close()
		ppty = nil
	}
	return err
}

func (e *hostExecutor) Exec2(command []string, cmdline string, env map[string]string, user, workdir string) common.Executor {
	return func(ctx context.Context) error {
		if err := e.exec2(ctx, command, cmdline, env, workdir, user); err != nil {
			select {
			case <-ctx.Done():
				return errors.Wrapf(err, "This step was cancelled\n")
			default:
				return err
			}
		}
		return nil
	}
}

func (e *hostExecutor) Exec(command []string, env map[string]string, user, workdir string) common.Executor {
	return func(ctx context.Context) error {
		if err := e.exec2(ctx, command, "", env, workdir, user); err != nil {
			select {
			case <-ctx.Done():
				return errors.Wrapf(err, "This step was cancelled\n")
			default:
				return err
			}
		}
		return nil
	}
}

var _singleLineEnvPattern *regexp.Regexp
var _mulitiLineEnvPattern *regexp.Regexp

func (e *hostExecutor) UpdateFromEnv(srcPath string, env *map[string]string) common.Executor {
	if _singleLineEnvPattern == nil {
		_singleLineEnvPattern = regexp.MustCompile("^([^=]+)=([^=]+)$")
		_mulitiLineEnvPattern = regexp.MustCompile(`^([^<]+)<<(\w+)$`)
	}

	localEnv := *env
	return func(ctx context.Context) error {
		envTar, err := e.GetContainerArchive(ctx, srcPath)
		if err != nil {
			return nil
		}
		defer envTar.Close()
		reader := tar.NewReader(envTar)
		_, err = reader.Next()
		if err != nil && err != io.EOF {
			return errors.WithStack(err)
		}
		s := bufio.NewScanner(reader)
		multiLineEnvKey := ""
		multiLineEnvDelimiter := ""
		multiLineEnvContent := ""
		for s.Scan() {
			line := s.Text()
			if singleLineEnv := _singleLineEnvPattern.FindStringSubmatch(line); singleLineEnv != nil {
				localEnv[singleLineEnv[1]] = singleLineEnv[2]
			}
			if line == multiLineEnvDelimiter {
				localEnv[multiLineEnvKey] = multiLineEnvContent
				multiLineEnvKey, multiLineEnvDelimiter, multiLineEnvContent = "", "", ""
			}
			if multiLineEnvKey != "" && multiLineEnvDelimiter != "" {
				if multiLineEnvContent != "" {
					multiLineEnvContent += "\n"
				}
				multiLineEnvContent += line
			}
			if mulitiLineEnvStart := _mulitiLineEnvPattern.FindStringSubmatch(line); mulitiLineEnvStart != nil {
				multiLineEnvKey = mulitiLineEnvStart[1]
				multiLineEnvDelimiter = mulitiLineEnvStart[2]
			}
		}
		env = &localEnv
		return nil
	}
}

func (e *hostExecutor) UpdateFromPath(env *map[string]string) common.Executor {
	localEnv := *env
	return func(ctx context.Context) error {
		pathTar, err := e.GetContainerArchive(ctx, localEnv["GITHUB_PATH"])
		if err != nil {
			return errors.WithStack(err)
		}
		defer pathTar.Close()

		reader := tar.NewReader(pathTar)
		_, err = reader.Next()
		if err != nil && err != io.EOF {
			return errors.WithStack(err)
		}
		s := bufio.NewScanner(reader)
		for s.Scan() {
			line := s.Text()
			pathSep := string(filepath.ListSeparator)
			localEnv["PATH"] = fmt.Sprintf("%s%s%s", line, pathSep, localEnv["PATH"])
		}

		env = &localEnv
		return nil
	}
}

func (e *hostExecutor) Remove() common.Executor {
	return func(ctx context.Context) error {
		return os.RemoveAll(e.input.WorkingDir)
	}
}
