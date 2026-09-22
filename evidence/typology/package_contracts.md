# Package public contracts

Exported symbols and delivery facts from static analysis.
Use packageDoc, methods, jsonTags, goEmbed, deliveryHint, and observed role (never folder names) to classify packages.

## ./cmd/majordomo
- package: `main`
- language: go
- packageDoc: Command majordomo is the control-plane CLI for repository operations.
- hasMain: true
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- deliveryHint: cli
- role: entrypoint
- confidence: 0.90
- roleEvidence: has_main
- exportedDecls: (none)
- exportedFuncs: (none)
- exportedMethods: (none)

## ./internal/agent
- package: `agent`
- language: go
- packageDoc: Package agent owns review dispatch helpers.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: DispatchOptions, Mode, ModeFiles, ModeFinalize, ModeProse, ModeScore, ModeSummary, ModeTechScore, ModeTechnical, ModeTechnicalDeep, SummaryLoopOptions, TechDeepOptions, TechLoopOptions
- exportedFuncs: Dispatch, FindScript, GroundingPaths, GroundingSkillDir, Logf, OpenCodeEnv, ParseRisksByFile, ParseScore, ResolveScriptsDir, RunOpenCode, RunSummaryLoop, RunTechDeep, RunTechLoop
- exportedMethods: (none)

## ./internal/agenting
- package: `agenting`
- language: go
- packageDoc: Package agenting loads and selects context-branch grounding packs for review prep.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: GroundingName, Index, IndexRelPath, ModeDigest, ModeFiles, ModeSummary, ModeTechnical, Pack, StagedPack
- exportedFuncs: LoadIndex, MatchGlob, ModeForSkill, Select, Stage, ValidateIndex
- exportedMethods: Index.PackIDs

## ./internal/aigateway
- package: `aigateway`
- language: go
- packageDoc: Package aigateway embeds Bifrost as an in-process LLM gateway.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: true
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- deliveryHint: server-http
- role: server
- confidence: 0.90
- roleEvidence: delivery:http, imports_net_http, http_route_register
- exportedDecls: Account, DummyAPIKey, Gateway
- exportedFuncs: Ensure, LogicalModel, NewAccountFromEnv, PrepareChildEnv, ResetForTests, ShutdownGlobal, Start
- exportedMethods: Account.GetConfigForProvider, Account.GetConfiguredProviders, Account.GetKeysForProvider, Account.HasProviders, Account.Providers, Gateway.BaseURL, Gateway.ChildEnv, Gateway.Origin, Gateway.Shutdown

## ./internal/cache
- package: `cache`
- language: go
- packageDoc: Package cache reads and writes review-cache, poll-cursor, and digest-inference data on the served repo (branches majordomo-inference-cache/<id> with path prefixes review/ and digest/, plus majordomo-poll-cache/<id>).
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: ClusterAuditCached, ClusterAuditCachedMerge, ClusterAuditFingerprint, ClusterCoTCached, ClusterCoTFingerprint, DigestCachePrefix, DigestClusterAuditPromptV1, DigestClusterAuditSchemaV1, DigestClusterCoTPromptV1, DigestClusterCoTSchemaV1, DigestClusterCoTSchemaV2, DigestClusterCoTSchemaV3, DigestInspectPromptV1, DigestInspectSchemaV1, DigestInspectSchemaV2, DigestInterventionPromptV1, DigestInterventionSchemaV1, DigestInterventionSchemaV2, DigestLedgerPromptV1, DigestLedgerSchemaV1, DigestLedgerSchemaV2, DigestPushOptions, DigestRefinePromptV1, DigestRefinePromptV2, DigestRefineSchemaV1, DigestRefineSchemaV2, DigestRefineSchemaV3, DigestRefineSchemaV4, DigestRefineSchemaV5, DigestRunStats, DigestStore, DigestStoryPromptV1, DigestStoryPromptV2, DigestStorySchemaV1, DigestStorySchemaV2, DigestStorySchemaV3, ExitPatternViolation, InspectCachedRole, InspectFingerprint, InterventionCached, InterventionFingerprint, LedgerCachedEntry, LedgerFingerprint, LookupOptions, Meta, PollCursor, PrecheckOptions, PushOptions, RefineCached, RefineFingerprint, RestoreOptions, ReviewCachePrefix, StoreOptions, StoryCached, StoryFingerprint
- exportedFuncs: AnalysisCacheName, ClusterFilesHash, ContentSHA, CursorPath, FormatStatsLine, HashDigestParts, Lookup, NormalizeClusterFiles, OwnedPackagesSourceHash, OwnedPathsHash, PackageSourceHash, Precheck, PrintJSON, PrintJSONPretty, Push, PushDigest, ReadPollCursor, RecordHead, Restore, ShouldReview, Store, ValidateDigestCacheBranch, ValidateInferenceCacheBranch, ValidatePollCacheBranch, ValidateReviewCacheBranch, WritePollCursor
- exportedMethods: DigestStore.ConfigurePush, DigestStore.Flush, DigestStore.LookupClusterAudit, DigestStore.LookupClusterCoT, DigestStore.LookupInspect, DigestStore.LookupIntervention, DigestStore.LookupLedger, DigestStore.LookupRefine, DigestStore.LookupStory, DigestStore.RecordClusterAuditHit, DigestStore.RecordClusterAuditMiss, DigestStore.RecordClusterCoTHit, DigestStore.RecordClusterCoTMiss, DigestStore.RecordInspectHit, DigestStore.RecordInspectMiss, DigestStore.RecordInterventionHit, DigestStore.RecordInterventionMiss, DigestStore.RecordLedgerHit, DigestStore.RecordLedgerMiss, DigestStore.RecordRefineHit, DigestStore.RecordRefineMiss, DigestStore.RecordStoryHit, DigestStore.RecordStoryMiss, DigestStore.Stats, DigestStore.StoreClusterAudit, DigestStore.StoreClusterCoT, DigestStore.StoreInspect, DigestStore.StoreIntervention, DigestStore.StoreLedger, DigestStore.StoreRefine, DigestStore.StoreStory

## ./internal/cli
- package: `cli`
- language: go
- packageDoc: Package cli wires majordomo subcommands.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: cli
- confidence: 0.80
- roleEvidence: cli_flag_parse, cli_subcommand, imports_cli_framework
- exportedDecls: Version
- exportedFuncs: NewRoot
- exportedMethods: (none)

## ./internal/cluster
- package: `cluster`
- language: go
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: UnionFind
- exportedFuncs: BuildCorpusIndex, ClusterDocs, ClusterFiles, DepClusterAwareBatches, DocClusterAwareBatches, NewUnionFind, ReverseDeps, ReverseLinks
- exportedMethods: UnionFind.Components, UnionFind.Find, UnionFind.Union

## ./internal/config
- package: `config`
- language: go
- packageDoc: Package config loads and merges majordomo-central-config YAML.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: AIProviderConfig, Cache, Context, ContextCompaction, DefaultsFilename, GroundingConfig, JobConfig, JobContextDigest, JobPRReview, MaterializeResult, ModuleTaskConfig, Observability, OrderedRouting, Pipeline, PipelineRoutingEntry, PollCache, PushNone, PushWebhook, PushWorkflow, RepoConfig, Repository, Review, SCMAPI, StaticAnalysisTool, Trigger, TriggerPush, TriggerPushMode
- exportedFuncs: ApplyPipelineModelEnv, CacheBranch, ContextBranch, ContextUpdateBranch, CredentialEnvName, CredentialHint, DigestCacheBranch, InferenceCacheBranch, JobForTask, ListRepoIDs, LoadAll, LoadDefaults, LoadMerged, LoadRepoFile, MaterializeDirForStaging, MaterializePrep, OrgCredentialEnvName, PollCacheBranch, ResolveCredential, ResolvePrepPaths, ResolveSAImage, ResolveSAToolSlug, TypologyPromoteBranch, TypologyPromoteUpdateBranch
- exportedMethods: AIProviderConfig.GetTimeout, AIProviderConfig.ToStrop, AIProviderConfig.UsesEmbeddedGateway, AIProviderConfig.Validate, Cache.SkipsEnabled, Context.AutoMergeEnabled, Context.GatePrefix, Context.MaxCommitsPerRunLimit, Observability.Expand, OrderedRouting.Empty, OrderedRouting.UnmarshalYAML, RepoConfig.EffectivePublishMode, RepoConfig.GetAIProvider, RepoConfig.GetModuleProvider, RepoConfig.PipelineNamed, RepoConfig.ResolveTaskProvider, Review.ContinuousRunsEnabled, Trigger.PollEnabled

## ./internal/contextdigest
- package: `contextdigest`
- language: go
- packageDoc: Package contextdigest runs the served-repo context catch-up job.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: true
- importsOsExec: true
- importsOtel: true
- importsPrometheus: false
- role: observability
- confidence: 0.90
- roleEvidence: imports_otel
- exportedDecls: BootstrapStoryGenerator, BootstrapStoryInput, BootstrapStoryOutput, BootstrapSurveyInput, BootstrapSurveyRunner, CommitContext, CompactOptions, FindingCommentBodiesFile, FindingCommentBody, FindingCommentsSidecar, Forge, Git, HumanInterventionGenerator, HumanInterventionInput, HumanInterventionOutput, JudgeBootstrapStoryGenerator, JudgeHumanInterventionGenerator, JudgeTypologySlicePipeline, LocalBootstrapSurveyRunner, LocalSeedManifest, LocalSeedWorkspace, LocalStageCatalog, LocalStageIntervention, LocalStageStory, LocalStageSurvey, Options, PRCommentAPI, PRCommentWithID, PRHead, RepoTarget, ReposResult, Result, ResumeProvenance, ResumeStageCatalog, ResumeStageIntervention, ResumeStageStory, RewriteInfo, TypologySlicePipeline, TypologySlicePipelineInput, TypologySlicePipelineOutput
- exportedFuncs: ApplyRewriteWhy, BeginRewrite, CheckoutBranch, CheckoutOrCreate, CheckoutUpdateBranch, CollectChangedFiles, CommitAll, CompactChronology, CompleteRewrite, DeepenDefaultBranch, DefaultCompactOptions, DetectRewrite, EnsureAncestor, FetchOrigin, FirstParentCommits, InitOrphan, IsAncestor, IsBehind, ListTargets, LoadCommitContext, LocalBranchExists, MaterializeAgenting, NeedsMetaUpdate, OpenLocalSeedWorkspace, ProcessCommit, Push, PushForce, ReadCursor, RemoteBranchExists, ReshapeStory, ResolveDefaultBranch, Run, SyncFindingPRComments, UpdateMeta, WalkCommits
- exportedMethods: Forge.ListPRComments, Forge.ListPRCommentsWithIDs, Forge.MergeUpdatePR, Forge.OpenUpdatePR, Forge.PostPRComment, Forge.ResolvePRHead, Forge.UpdatePRComment, Forge.WriteTipBranch, JudgeBootstrapStoryGenerator.Generate, JudgeHumanInterventionGenerator.Generate, JudgeTypologySlicePipeline.Assemble, LocalBootstrapSurveyRunner.Survey, LocalSeedWorkspace.AnalysisDir, LocalSeedWorkspace.BeginStage, LocalSeedWorkspace.CacheDir, LocalSeedWorkspace.ContextDir, LocalSeedWorkspace.DiffPath, LocalSeedWorkspace.IsNew, LocalSeedWorkspace.PersistAnalysisDrafts, LocalSeedWorkspace.RecordFailure, LocalSeedWorkspace.Release, LocalSeedWorkspace.RestoreAnalysisDrafts, LocalSeedWorkspace.SaveCheckpoint, LocalSeedWorkspace.WorkStoryDir, LocalSeedWorkspace.WriteLocalDiff, digestSectionRunner.Run, ledgerGroundingIssue.Error, ledgerStepRunner.RunStep, packageJudgeGenerator.Evaluate, packageJudgeGenerator.Generate, packageJudgeGenerator.Ready, packageJudgeGenerator.TaskModel, rlmBootstrapStoryGenerator.Generate, rlmCompleteAdapter.Complete, stropBootstrapStoryRLM.Complete, stropClusterMergeAuditor.Audit, stropClusterMergeAuditor.Complete, stropClusterMergeAuditor.TraceDir, stropPackageRoleRLM.Validate, stropSliceCatalogRLM.AssembleSlices, stropSliceCatalogRLM.Complete, stropSliceObjectiveLedgerRLM.BuildSliceLedger, stropSliceObjectiveLedgerRLM.Complete, stubSliceLedgerCaller.Complete

## ./internal/contextgate
- package: `contextgate`
- language: go
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: Action, ActionDone, ActionIgnore, ActionReject, ActionWhy, Comment, DefaultPrefix, ErrNotImplemented, FileStore, Gate, ParsedComment, Sidecar, Status, StatusBlockedWhy, StatusDone, StatusOpen, StatusRejected
- exportedFuncs: ApplyComments, LoadSidecar, NewGate, ParseComment, RegenOptions, SaveSidecar, SidecarPath, SyncFromComments
- exportedMethods: FileStore.AppendStatus, FileStore.Load, FileStore.Save, Gate.NormalizeReject, Sidecar.ReadyToMerge, Sidecar.RegenRequested

## ./internal/contextstore
- package: `contextstore`
- language: go
- packageDoc: Package contextstore validates the served-repo context branch tree (meta.yaml, chronology.md, and required dossier files).
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: ChronologyEvent, CurrentSchemaVersion, Meta, RequiredFiles, StoryReadingOrder, TypologyAppendixFiles, TypologyArchitectureBriefPath, TypologyManifest, TypologyModeDiscover, TypologyModeFallback, TypologyModeReuse, TypologyReadingIndexPath, TypologyReadingOrder, TypologyRefineComplete, TypologyRefinePending, TypologyRefineSkipped
- exportedFuncs: AppendChronologyEvent, ApplyReadingPath, Bootstrap, EnsureReadingNav, EnsureRootReadingTOC, ParseChronology, ParseChronologyFile, ParseMeta, ParseTypologyManifest, ValidateContextBranch, ValidateMeta, ValidateTree, ValidateTypologyManifest
- exportedMethods: (none)

## ./internal/diff
- package: `diff`
- language: go
- packageDoc: Package diff builds combined diffs from staging manifests.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- deliveryHint: dto
- role: dto
- confidence: 0.90
- roleEvidence: json_tags, decl_heavy_export_surface
- exportedDecls: BuildAllOptions
- exportedFuncs: BuildAll
- exportedMethods: (none)

## ./internal/filereview
- package: `filereview`
- language: go
- packageDoc: Package filereview is the Prepare → Judge → Validate → Assemble state machine for per-file PR review batches.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: validation
- confidence: 0.90
- roleEvidence: validate_export, validation_report
- exportedDecls: Finding, JudgeFunc, Options, Report, Reviewable, Severity, SeverityCritical, SeverityInfo, SeverityWarn
- exportedFuncs: Assemble, CollectReports, FormatMarkdown, LoadReviewables, ParseMarkdownReport, ParseSeverity, PerFileDir, Run, ValidateReports
- exportedMethods: Severity.Tag

## ./internal/githttps
- package: `githttps`
- language: go
- packageDoc: Package githttps builds git -c http.extraHeader args for forge HTTPS remotes.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: (none)
- exportedFuncs: ExtraHeaderArgs, InferSCM
- exportedMethods: (none)

## ./internal/judge
- package: `judge`
- language: go
- packageDoc: Package judge is the Majordomo boundary onto strop JobRunner and evaluation packs.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: pipeline
- confidence: 0.80
- roleEvidence: pipeline_registry_param, pipeline_runner_type
- exportedDecls: DispatchMode, DispatchModeFiles, DispatchModeFinalize, DispatchModeProse, DispatchModeScore, DispatchModeSummary, DispatchModeTechScore, DispatchModeTechnical, DispatchModeTechnicalDeep, DispatchOptions, ErrNotReady, ErrStropJudgeNotReady, FileReviewOptions, Generator, MinEvalPassScore, Runtime, RuntimeOptions
- exportedFuncs: AllGeneratorTasks, DefaultModuleRetryConfig, DefaultRuntime, DigestTasks, Dispatch, EnsureRuntimeFromConfig, EnsureStropReady, EvalFeedback, EvalPassed, Evaluate, FileReviewBatch, Generate, LLMConfigured, NewJobRunner, NewRuntime, RegisterPacks, ResetRegistryForTests, ResolveGatewayProvider, ResolveProvider, ReviewTasks, SetDefaultRuntime, SharedRunner, StoryLLMAvailable, StropReady, WrapLLMWithRetry
- exportedMethods: Runtime.Evaluate, Runtime.Generate, Runtime.Ready, Runtime.TaskModel, mapInput.EvaluationMap, mapInput.GetVersion, mapInput.ToMap, nopLogger.Debug, nopLogger.Error, nopLogger.Info, nopLogger.Warn, nopLogger.WithError, nopLogger.WithField, nopLogger.WithFields, retryLLM.Generate, retryLLM.GenerateWithContent, singleRoleInfo.ConsolidatorKey, singleRoleInfo.ConsolidatorName, singleRoleInfo.EvaluatorName, singleRoleInfo.EvaluatorWeight, singleRoleInfo.HasEvaluator

## ./internal/judge/evaluation/bootstrap
- package: `bootstrap`
- language: go
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: CriterionIDEvidencedOnly, CriterionIDHonestSeed, CriterionIDPreservesForm, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)

## ./internal/judge/evaluation/digest
- package: `digest`
- language: go
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: CriterionIDEvidencedOnly, CriterionIDPreservesForm, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)

## ./internal/judge/evaluation/summary
- package: `summary`
- language: go
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: CriterionIDCallerFacingH3s, CriterionIDH2Structure, CriterionIDJudgmentH3, CriterionIDNoEmDashConnectors, CriterionIDNoFilenamesInProse, CriterionIDNoGenericPhrases, CriterionIDNoPrescriptiveFix, CriterionIDTeamConsequence, CriterionIDWhatGotBuiltBlocks, CriterionIDWhyNamesArtifact, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)

## ./internal/judge/evaluation/tech
- package: `tech`
- language: go
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: CriterionIDBlankLinesFields, CriterionIDChecklistSkip, CriterionIDConfirmYesNo, CriterionIDDeclarativeH3, CriterionIDFourFields, CriterionIDH2Structure, CriterionIDNoEmDashConnectors, CriterionIDNoGenericPhrases, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)

## ./internal/judge/evaluation/typology
- package: `typology`
- language: go
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: ClusterCriterionIDs, CriterionIDAdapterSurfaces, CriterionIDClusterCounsel, CriterionIDClusterDelivery, CriterionIDDebtWhenFindings, CriterionIDInterventionCounsel, CriterionIDInterventionCoverage, CriterionIDInterventionNoInvent, CriterionIDInterventionStatus, CriterionIDInterventionTutorVoice, CriterionIDJourneyConsistent, CriterionIDJourneyCounsel, CriterionIDObjectives, CriterionIDRoleGrounding, CriterionIDSliceOwnership, CriterionIDSurfaces, CriterionIDs, FindingCommentCriterionIDs, InterventionBriefCriterionIDs, InterventionCriterionIDs, InterventionJourneyCriterionIDs, InterventionPRPriorityCriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)

## ./internal/judge/modules
- package: `modules`
- language: go
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: TaskBootstrapStory, TaskDigestStory, TaskFileReview, TaskSummary, TaskTechnical, TaskTypologyFindingComment, TaskTypologyHumanIntervention, TaskTypologyInspect, TaskTypologyInterventionBrief, TaskTypologyInterventionJourney, TaskTypologyInterventionPRPriority, TaskTypologyInterventionWeaknesses, TaskTypologySliceCatalog, TaskTypologySliceGrouping, TaskTypologySliceGroupingAudit, TaskTypologySliceMeaning
- exportedFuncs: BootstrapStoryModule, DigestStoryModule, FileReviewModule, SummaryModule, TechnicalModule, TypologyClusterModule, TypologyFindingCommentModule, TypologyHumanInterventionModule, TypologyInspectModule, TypologyInterventionBriefModule, TypologyInterventionJourneyModule, TypologyInterventionPRPriorityModule, TypologyInterventionWeaknessesModule, TypologyRefineModule
- exportedMethods: (none)

## ./internal/llmusage
- package: `llmusage`
- language: go
- packageDoc: Package llmusage aggregates provider-reported LLM token usage for a Majordomo run.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: Collector, Summary, TaskRow
- exportedFuncs: Active, Format, FromContext, New, Pop, Push, RecordExecutionState, WithCollector
- exportedMethods: Collector.Add, Collector.AddFromTokenUsage, Collector.AddTokenUsageValue, Collector.Snapshot

## ./internal/observability
- package: `observability`
- language: go
- packageDoc: Package observability provides OpenTelemetry tracing and inference failure dumps.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: true
- importsOsExec: false
- importsOtel: true
- importsPrometheus: false
- role: observability
- confidence: 0.90
- roleEvidence: imports_otel
- exportedDecls: Config, DefaultServiceName, Settings
- exportedFuncs: EndSpanWithStatus, Flush, Init, InstrumentHTTPClient, NewFailureDumpProcessorForTest, ResolveConfig, Shutdown, StartChainSpan, TraceIDFromContext, WrapRoundTripper
- exportedMethods: errorDetailTransport.RoundTrip, failureDumpProcessor.ForceFlush, failureDumpProcessor.OnEnd, failureDumpProcessor.OnStart, failureDumpProcessor.Shutdown, redactedError.Error, redactedError.Unwrap

## ./internal/orchestrate
- package: `orchestrate`
- language: go
- packageDoc: Package orchestrate runs review waves, checkpoints, finalize, and synthesis loops.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: BatchEntry, BatchPlan, Options, StageFinalize, StagePrep, StageProse, StageReport, StageSynth, StageWaves
- exportedFuncs: CheckpointPath, ChunkBatches, FileExists, IsSynthesisSkill, LoadBatchPlan, NormalizeUntil, Run, SplitBatches, TouchCheckpoint
- exportedMethods: (none)

## ./internal/outbound
- package: `outbound`
- language: go
- packageDoc: Package outbound provides a shared retrying HTTP client for forge/SCM APIs.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: true
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: DefaultTimeout
- exportedFuncs: Client, DoWithRetry
- exportedMethods: (none)

## ./internal/poll
- package: `poll`
- language: go
- packageDoc: Package poll discovers open PRs/MRs via SCM APIs and compares head_sha against a local poll cursor (.poll-cache; Actions cache in the tower).
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: true
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: Options, PendingReview, Result
- exportedFuncs: Run
- exportedMethods: (none)

## ./internal/publish
- package: `publish`
- language: go
- packageDoc: Package publish posts PR/MR summaries via forge CLIs (gh, glab) or Bitbucket HTTP.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: true
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: exec_runner
- confidence: 0.80
- roleEvidence: imports_os_exec, exports_run_surface
- exportedDecls: CLIRunner, Marker, Mode, ModeAuto, ModeComment, ModeDescription, Options
- exportedFuncs: HasBodyContent, Run
- exportedMethods: (none)

## ./internal/report
- package: `report`
- language: go
- packageDoc: Package report converts review findings to JUnit XML and Markdown reports to HTML.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: (none)
- exportedFuncs: BuildTestsuite, ConvertMarkdownToHTML, ConvertMarkdownToHTMLCLI, ConvertToJUnit, DeriveTitle, Sanitize
- exportedMethods: (none)

## ./internal/reviewrun
- package: `reviewrun`
- language: go
- packageDoc: Package reviewrun is the local/CI review job: clone, SA, orchestrate, optional publish.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: exec_runner
- confidence: 0.80
- roleEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options, StageClone, StageFinalize, StagePrep, StageProse, StagePublish, StageReport, StageSA, StageSynth, StageWaves
- exportedFuncs: ParseUntil, Run
- exportedMethods: (none)

## ./internal/sa
- package: `sa`
- language: go
- packageDoc: Package sa runs staticAnalysis tools from central config against changed files.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: exec_runner
- confidence: 0.80
- roleEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options, ToolRunner
- exportedFuncs: Run
- exportedMethods: (none)

## ./internal/satools
- package: `satools`
- language: go
- packageDoc: Package satools builds local SA tool Docker images for Dockerfile validation.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: exec_runner
- confidence: 0.80
- roleEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options
- exportedFuncs: Run
- exportedMethods: (none)

## ./internal/staging
- package: `staging`
- language: go
- packageDoc: Package staging ports git-diff-prep: classify, cluster, batch, write manifest.
- hasMain: false
- jsonTags: true
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: exec_runner
- confidence: 0.80
- roleEvidence: imports_os_exec, exports_run_surface
- exportedDecls: AgentContext, BatchEntry, CrossSkillBatchDir, CrossSkillBatchNum, DefaultRouting, ErrFatal, ErrNothingToReview, GitError, GitRunner, GitTimeout, MaxCombinedLines, MaxDiffLines, MaxStageFilenameBytes, ModeDiffChunk, ModeDiffOnly, ModeFullAndDiff, Options, RoutingRule, SetupGitResult, Task
- exportedFuncs: AttachGrounding, BuildStagingFilename, ChunkLines, ClassifyFile, CollectSAFindings, ContextForFile, DetectSADir, FileSlug, GetSubmoduleExclusions, IsExcluded, IsExcludedWithExtra, LoadAgentContextConfig, LoadRouting, LoadSummaryConfig, MatchGlob, ParseNameStatus, ParseSubmoduleStatusLines, ResolveContextDir, ResolveRoutingPersonas, Run, SetupGit, StageCrossSkillBatches, StageFile, StageSkillBatches, WriteBatchPlan
- exportedMethods: ErrFatal.Error, GitError.Error

## ./internal/status
- package: `status`
- language: go
- packageDoc: Package status posts commit/build status to GitHub and Bitbucket Server.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: true
- importsOsExec: false
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: Options, State, StateFailed, StateInProgress, StateSuccessful
- exportedFuncs: Run
- exportedMethods: (none)

## ./internal/submodule
- package: `submodule`
- language: go
- packageDoc: Package submodule ports scripts/submodule.py: interactive .majordomo manager.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: exec_runner
- confidence: 0.80
- roleEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options
- exportedFuncs: Run
- exportedMethods: (none)

## ./internal/workspace
- package: `workspace`
- language: go
- packageDoc: Package workspace is the cwd-bounded port for explore/edit tools.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: Allow, AllowDigest, AllowNone, AllowTechDeep, ErrDenied, ErrEscape, Local, Match, Port, Stub, Tool, ToolEdit, ToolGrep, ToolRead, ToolShell
- exportedFuncs: Guard, NewLocal, NewStub
- exportedMethods: Local.Edit, Local.Grep, Local.Read, Local.Shell, Stub.Edit, Stub.Grep, Stub.Read, Stub.Shell, Tool.String, guarded.Edit, guarded.Grep, guarded.Read, guarded.Shell

## ./internal/workspace/opencode
- package: `opencode`
- language: go
- packageDoc: Package opencode is the OpenCode CLI adapter for workspace.Port.
- hasMain: false
- jsonTags: false
- goEmbed: false
- importsNetHTTP: false
- importsOsExec: true
- importsOtel: false
- importsPrometheus: false
- role: unknown
- confidence: 0.00
- exportedDecls: Adapter, ErrNotImplemented
- exportedFuncs: New
- exportedMethods: Adapter.ChildEnv, Adapter.Edit, Adapter.Grep, Adapter.Read, Adapter.Shell

