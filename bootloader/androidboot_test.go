// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2017 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package bootloader_test

import (
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/bootloader"
	"github.com/snapcore/snapd/dirs"
	"github.com/snapcore/snapd/osutil"
	"github.com/snapcore/snapd/snap"
	"github.com/snapcore/snapd/snap/snaptest"
	"github.com/snapcore/snapd/testutil"
)

const packageKernel = `
name: ubuntu-kernel
version: 4.0-1
type: kernel
vendor: Someone
`

type androidBootTestSuite struct {
	testutil.BaseTest
}

var _ = Suite(&androidBootTestSuite{})

func (g *androidBootTestSuite) SetUpTest(c *C) {
	g.BaseTest.SetUpTest(c)
	g.BaseTest.AddCleanup(snap.MockSanitizePlugsSlots(func(snapInfo *snap.Info) {}))
	dirs.SetRootDir(c.MkDir())

	// the file needs to exist for androidboot object to be created
	bootloader.MockAndroidBootFile(c, 0644)
}

func (g *androidBootTestSuite) TearDownTest(c *C) {
	g.BaseTest.TearDownTest(c)
	dirs.SetRootDir("")
}

func (s *androidBootTestSuite) TestNewAndroidbootNoAndroidbootReturnsNil(c *C) {
	dirs.GlobalRootDir = "/something/not/there"
	a := bootloader.NewAndroidBoot()
	c.Assert(a, IsNil)
}

func (s *androidBootTestSuite) TestNewAndroidboot(c *C) {
	a := bootloader.NewAndroidBoot()
	c.Assert(a, NotNil)
}

func (s *androidBootTestSuite) TestSetGetBootVar(c *C) {
	a := bootloader.NewAndroidBoot()
	bootVars := map[string]string{"snap_mode": "try"}
	a.SetBootVars(bootVars)

	v, err := a.GetBootVars("snap_mode")
	c.Assert(err, IsNil)
	c.Check(v, HasLen, 1)
	c.Check(v["snap_mode"], Equals, "try")
}

func (s *androidBootTestSuite) TestExtractKernelAssetsNoUnpacksKernel(c *C) {
	a := bootloader.NewAndroidBoot()

	c.Assert(a, NotNil)

	files := [][]string{
		{"kernel.img", "I'm a kernel"},
		{"initrd.img", "...and I'm an initrd"},
		{"meta/kernel.yaml", "version: 4.2"},
	}
	si := &snap.SideInfo{
		RealName: "ubuntu-kernel",
		Revision: snap.R(42),
	}
	fn := snaptest.MakeTestSnapWithFiles(c, packageKernel, files)
	snapf, err := snap.Open(fn)
	c.Assert(err, IsNil)

	info, err := snap.ReadInfoFromSnapFile(snapf, si)
	c.Assert(err, IsNil)

	err = a.ExtractKernelAssets(info, snapf)
	c.Assert(err, IsNil)

	// kernel is *not* here
	kernimg := filepath.Join(a.Dir(), "ubuntu-kernel_42.snap", "kernel.img")
	c.Assert(osutil.FileExists(kernimg), Equals, false)
}
