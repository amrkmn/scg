package service

import (
	"fmt"
	"sync"
	"time"

	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
)

// installPlan is the brew-faithful split of installCore: everything the
// mechanical stage (download/hash/extract) and the sequential link phase
// need, resolved up front. Pure parse: no downloads, no filesystem writes,
// no hooks run while building a plan.
type installPlan struct {
	m                                *scoop.Manifest
	appName, bucketName, displayName string
	arch                             string
	opts                             InstallOptions
	logger                           Logger
	verbose                          bool
	paths                            scoop.ScoopPaths
	appDir, versionDir               string
	currentLink, currentDir          string
	persistDir, downloadFile         string
	binDefs                          []install.ShimDef
	preInstall, postInstall          string
	persistItems                     []install.PersistItem
	envPaths                         []string
	envVars                          map[string]string
	preHookEnv                       map[string]string
	expandedEnvVars                  map[string]string
	shortcuts                        []install.ShortcutDef
	dlURLs                           []string
	firstURL                         string
}

// installStaged is the output of the parallel stage: cache paths for every
// URL plus how long the stage took. The link phase consumes it sequentially.
type installStaged struct {
	dlResults []*install.DownloadResult
}

// stepLogger is a quiet-capable facade over Logger plus the step helpers.
// Workers in the parallel stage run quiet (like brew's download-queue
// workers, which stay quiet while progress lines redraw); the sequential
// link phase runs loud. quiet=false delegates 1:1 to the globals/logger,
// so legacy single-app output is byte-identical.
type stepLogger struct {
	inner Logger
	quiet bool
}

func (l *stepLogger) Log(msg string) {
	if !l.quiet {
		l.inner.Log(msg)
	}
}
func (l *stepLogger) Info(msg string) {
	if !l.quiet {
		l.inner.Info(msg)
	}
}
func (l *stepLogger) Success(msg string) {
	if !l.quiet {
		l.inner.Success(msg)
	}
}
func (l *stepLogger) Warn(msg string) {
	if !l.quiet {
		l.inner.Warn(msg)
	}
}
func (l *stepLogger) Error(msg string) {
	if !l.quiet {
		l.inner.Error(msg)
	}
}
func (l *stepLogger) Verbose(msg string) {
	if !l.quiet {
		l.inner.Verbose(msg)
	}
}
func (l *stepLogger) Header(msg string) {
	if !l.quiet {
		l.inner.Header(msg)
	}
}
func (l *stepLogger) Detail(msg string) {
	if !l.quiet {
		l.inner.Detail(msg)
	}
}
func (l *stepLogger) Done(s, d string) {
	if !l.quiet {
		l.inner.Done(s, d)
	}
}
func (l *stepLogger) Skip(s, d string) {
	if !l.quiet {
		l.inner.Skip(s, d)
	}
}
func (l *stepLogger) Dry(s, d string) {
	if !l.quiet {
		l.inner.Dry(s, d)
	}
}
func (l *stepLogger) Newline() {
	if !l.quiet {
		l.inner.Newline()
	}
}
func (l *stepLogger) stepStart(t string) {
	if !l.quiet {
		logStepStart(t)
	}
}
func (l *stepLogger) stepDone() {
	if !l.quiet {
		logStepDone()
	}
}
func (l *stepLogger) stepError() {
	if !l.quiet {
		logStepError()
	}
}
func (l *stepLogger) stepOK() {
	if !l.quiet {
		logStepOK()
	}
}
func (l *stepLogger) hookStart(t string) {
	if !l.quiet {
		logHookStepStart(t)
	}
}
func (l *stepLogger) hookDone(m string) {
	if !l.quiet {
		logHookStepDone(m)
	}
}
func (l *stepLogger) hookError() {
	if !l.quiet {
		logHookStepError()
	}
}

// installDeps installs an app's dependencies via the recursive Install flow.
// Shared by legacy installCore and the batched driver so both agree.
func (s *InstallService) installDeps(m *scoop.Manifest, opts InstallOptions) error {
	if opts.Independent {
		return nil
	}
	deps := scoop.GetDependencies(m.Depends)
	if len(deps) == 0 {
		return nil
	}
	depOpts := opts
	depOpts.AsDependencyFlow = true
	for _, dr := range s.Install(deps, depOpts) {
		if !dr.Success && !dr.Skipped {
			return fmt.Errorf("dependency %s failed to install: %w", dr.App, dr.Error)
		}
	}
	return nil
}

// phasedInstallEligible mirrors the old prefetch condition: multi-app,
// real (non-dry) installs where downloads may hit the network. Dependency
// flows and SkipDownload (update internals) stay on the legacy path.
func phasedInstallEligible(apps []string, opts InstallOptions) bool {
	return len(apps) > 1 && !opts.DryRun && !opts.SkipDownload && !opts.AsDependencyFlow
}

// dedupeInputs drops exact duplicate inputs, preserving first-seen order.
func dedupeInputs(apps []string) []string {
	seen := make(map[string]struct{}, len(apps))
	out := make([]string, 0, len(apps))
	for _, a := range apps {
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

// displayInstallResult prints one result line. Extracted from the Install
// loop so the batched driver reports identically.
func (s *InstallService) displayInstallResult(result InstallResult, opts InstallOptions) {
	if opts.AsDependencyFlow {
		if result.Skipped {
			s.ctx.GetLogger().Skip(result.App, "already installed")
		} else if result.Success {
			s.ctx.GetLogger().Done(result.App, "installed")
		} else if result.Error != nil {
			s.ctx.GetLogger().Error(fmt.Sprintf("%s: %v", result.App, result.Error))
		}
	} else {
		if result.Skipped {
			s.ctx.GetLogger().Skip(result.App, fmt.Sprintf("%s already installed", result.Version))
		} else if result.Success {
			s.ctx.GetLogger().Done(result.App, fmt.Sprintf("installed %s", result.Version))
			s.ctx.GetLogger().Verbose(fmt.Sprintf("duration: %s", result.Duration.Round(time.Millisecond)))
		} else if result.Error != nil {
			s.ctx.GetLogger().Error(fmt.Sprintf("%s: %v", result.App, result.Error))
		}
	}
}

// installPending pairs a built plan with its in-progress result shell.
type installPending struct {
	plan   *installPlan
	result InstallResult
}

// stageOut is one parallel-stage result, index-aligned with its plan.
type stageOut struct {
	staged *installStaged
	dur    time.Duration
	err    error
}

// stagePlans runs the mechanical stage for every plan on a bounded pool
// governed by --jobs (same knob brew uses for its download+stage pool).
// Workers are quiet; results come back index-aligned for the sequential link.
func (s *InstallService) stagePlans(plans []*installPlan, quiet bool, concurrency int) []stageOut {
	out := make([]stageOut, len(plans))
	if len(plans) == 0 {
		return out
	}
	if concurrency < 1 {
		concurrency = DefaultDownloadConcurrency
	}
	if concurrency > len(plans) {
		concurrency = len(plans)
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, p := range plans {
		wg.Add(1)
		go func(idx int, plan *installPlan) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			start := time.Now()
			staged, err := s.stageInstallPlan(plan, quiet)
			out[idx] = stageOut{staged: staged, dur: time.Since(start), err: err}
		}(i, p)
	}
	wg.Wait()
	return out
}

// installBatched is the phased driver: resolve sequentially (mirroring
// InstallSingle's manifest/skip/warn semantics), stage in parallel while
// quiet, then link sequentially in input order.
func (s *InstallService) installBatched(inputs []string, opts InstallOptions) []InstallResult {
	if opts.Scope == "" {
		opts.Scope = scoop.ScopeUser
	}
	results := make([]InstallResult, 0, len(inputs))
	var pendings []installPending

	for _, input := range inputs {
		result := InstallResult{App: input}
		arch := opts.Arch
		if arch == "" {
			arch = detectArch()
		}
		_, appName := parseBucketAndApp(input)
		lookupName := input
		if opts.FromBucket != "" {
			lookupName = fmt.Sprintf("%s/%s", opts.FromBucket, appName)
		}
		installed, bucket := s.manifests.FindManifestPair(lookupName)
		var m *scoop.Manifest
		var bucketName string
		if bucket != nil {
			m = bucket.Manifest
			bucketName = bucket.Bucket
		} else if installed != nil {
			result.Skipped = true
			result.Version = installed.Manifest.Version
			results = append(results, result)
			s.displayInstallResult(result, opts)
			continue
		} else {
			result.Error = fmt.Errorf("app %q not found in any bucket", input)
			results = append(results, result)
			s.displayInstallResult(result, opts)
			continue
		}
		result.Version = m.Version

		requestedBucket, _ := parseBucketAndApp(input)
		if requestedBucket == "" && opts.FromBucket == "" {
			matchCount := 0
			for _, fm := range s.manifests.FindAllManifests(input) {
				if fm.Source == "bucket" {
					matchCount++
				}
			}
			if matchCount > 1 {
				s.ctx.GetLogger().Warn(fmt.Sprintf("Multiple buckets contain manifest '%s', the current selection is '%s/%s'.", appName, bucketName, appName))
			}
		}

		if installed != nil {
			if installed.Bucket == bucketName && installed.Manifest.Version == m.Version {
				result.Skipped = true
				results = append(results, result)
				s.displayInstallResult(result, opts)
				continue
			}
		}

		s.ctx.GetLogger().Header(fmt.Sprintf("Installing %s %s [%s] from %s", input, m.Version, arch, bucketName))

		if err := s.installDeps(m, opts); err != nil {
			result.Error = err
			results = append(results, result)
			s.displayInstallResult(result, opts)
			continue
		}
		plan, err := s.buildInstallPlan(m, appName, bucketName, input, arch, opts)
		if err != nil {
			result.Error = err
			results = append(results, result)
			s.displayInstallResult(result, opts)
			continue
		}
		pendings = append(pendings, installPending{plan: plan, result: result})
	}

	staged := s.stagePlans(pendingPlans(pendings), true, ResolveDownloadConcurrency(opts.DownloadConcurrency))
	for i, p := range pendings {
		r := p.result
		if staged[i].err != nil {
			r.Error = staged[i].err
			results = append(results, r)
			s.displayInstallResult(r, opts)
			continue
		}
		start := time.Now()
		r = s.linkInstallPlan(p.plan, staged[i].staged)
		r.Duration = staged[i].dur + time.Since(start)
		results = append(results, r)
		s.displayInstallResult(r, opts)
	}
	return results
}

func pendingPlans(pendings []installPending) []*installPlan {
	plans := make([]*installPlan, 0, len(pendings))
	for _, p := range pendings {
		plans = append(plans, p.plan)
	}
	return plans
}
