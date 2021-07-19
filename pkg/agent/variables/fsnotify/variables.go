package fsnotify

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/deviceplane/agent/pkg/agent/variables"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/crypto/ssh"
)

type Variables struct {
	dir  string
	lock sync.RWMutex

	disableSSH               bool
	disableSSHSet            bool
	authorizedSSHKeys        []ssh.PublicKey
	authorizedSSHKeysSet     bool
	hostSignerKey            string
	hostSignerKeySet         bool
	registryAuth             string
	registryAuthSet          bool
	whitelistedImages        []string
	whitelistedImagesSet     bool
	disableCustomCommands    bool
	disableCustomCommandsSet bool
}

func NewVariables(dir string) *Variables {
	return &Variables{
		dir: dir,
	}
}

func (v *Variables) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	v.refresh()

	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return
				}
				v.refresh()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.WithError(err).Error("variables watcher error")
			}
		}
	}()

	return watcher.Add(v.dir)
}

func (v *Variables) refresh() {
	for _, refresher := range []func() error{
		v.refreshDisableSSH,
		v.refreshAuthorizedSSHKeys,
		v.refreshHostSignerKey,
		v.refreshRegistryAuth,
		v.refreshWhitelistedImages,
		v.refreshDisableCustomCommands,
	} {
		if err := refresher(); err != nil {
			log.WithError(err).Error("variables refresh")
		}
	}
}

func (v *Variables) refreshDisableSSH() error {
	_, err := os.Stat(path.Join(v.dir, variables.DisableSSH))

	v.lock.Lock()
	defer v.lock.Unlock()

	if err == nil {
		v.disableSSH = true
		v.disableSSHSet = true
	} else if os.IsNotExist(err) {
		v.disableSSH = false
		v.disableSSHSet = true
	} else {
		return err
	}

	return nil
}

func (v *Variables) refreshAuthorizedSSHKeys() error {
	bytes, err := ioutil.ReadFile(path.Join(v.dir, variables.AuthorizedSSHKeys))

	v.lock.Lock()
	defer v.lock.Unlock()

	if err == nil {
		authorizedSSHKeys, err := parseAuthorizedKeysFile(bytes)
		if err != nil {
			return err
		}
		v.authorizedSSHKeys = authorizedSSHKeys
		v.authorizedSSHKeysSet = true
	} else if os.IsNotExist(err) {
		v.authorizedSSHKeys = make([]ssh.PublicKey, 0)
		v.authorizedSSHKeysSet = true
	} else {
		return err
	}

	return nil
}

func (v *Variables) refreshHostSignerKey() error {
	bytes, err := ioutil.ReadFile(path.Join(v.dir, variables.HostSignerKey))

	v.lock.Lock()
	defer v.lock.Unlock()

	if err == nil {
		v.hostSignerKey = string(bytes)
		v.hostSignerKeySet = true
	} else if os.IsNotExist(err) {
		v.hostSignerKey = ""
		v.hostSignerKeySet = true
	} else {
		return err
	}

	return nil
}

func (v *Variables) refreshRegistryAuth() error {
	bytes, err := ioutil.ReadFile(path.Join(v.dir, variables.RegistryAuth))

	v.lock.Lock()
	defer v.lock.Unlock()

	if err == nil {
		v.registryAuth = strings.TrimSpace(string(bytes))
		v.registryAuthSet = true
	} else if os.IsNotExist(err) {
		v.registryAuth = ""
		v.registryAuthSet = true
	} else {
		return err
	}

	return nil
}

func (v *Variables) refreshWhitelistedImages() error {
	bytes, err := ioutil.ReadFile(path.Join(v.dir, variables.WhitelistedImages))

	v.lock.Lock()
	defer v.lock.Unlock()

	if err == nil {
		v.whitelistedImages = []string{}
		nonCleanedImages := strings.Split(string(bytes), "\n")
		for _, image := range nonCleanedImages {
			cleanedImage := strings.TrimSpace(image)
			if len(cleanedImage) != 0 {
				v.whitelistedImages = append(v.whitelistedImages, cleanedImage)
			}
		}

		v.whitelistedImagesSet = true
	} else if os.IsNotExist(err) {
		v.whitelistedImages = []string{}
		v.whitelistedImagesSet = true
	} else {
		return err
	}

	return nil
}

func (v *Variables) refreshDisableCustomCommands() error {
	_, err := os.Stat(path.Join(v.dir, variables.DisableCustomCommands))

	v.lock.Lock()
	defer v.lock.Unlock()

	if err == nil {
		v.disableCustomCommands = true
		v.disableCustomCommandsSet = true
	} else if os.IsNotExist(err) {
		v.disableCustomCommands = false
		v.disableCustomCommandsSet = true
	} else {
		return err
	}

	return nil
}

func (v *Variables) GetDisableSSH() bool {
	v.waitFor(func() bool {
		return v.disableSSHSet
	})
	return v.disableSSH
}

func (v *Variables) GetAuthorizedSSHKeys() []ssh.PublicKey {
	v.waitFor(func() bool {
		return v.authorizedSSHKeysSet
	})
	return v.authorizedSSHKeys
}

func (v *Variables) GetHostSignerKey() string {
	v.waitFor(func() bool {
		return v.hostSignerKeySet
	})
	return v.hostSignerKey
}

func (v *Variables) GetRegistryAuth() string {
	v.waitFor(func() bool {
		return v.registryAuthSet
	})
	return v.registryAuth
}

func (v *Variables) GetWhitelistedImages() []string {
	v.waitFor(func() bool {
		return v.whitelistedImagesSet
	})
	return v.whitelistedImages
}

func (v *Variables) GetDisableCustomCommands() bool {
	v.waitFor(func() bool {
		return v.disableCustomCommandsSet
	})
	return v.disableCustomCommands
}

func (v *Variables) waitFor(getField func() bool) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		v.lock.RLock()
		field := getField()
		v.lock.RUnlock()
		if field {
			return
		}
		<-ticker.C
	}
}
