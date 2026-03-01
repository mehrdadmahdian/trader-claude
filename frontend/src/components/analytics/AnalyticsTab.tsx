import { useState } from 'react'
import { ChevronDown, ChevronRight, Loader2 } from 'lucide-react'
import { useStartParamHeatmap, useStartMonteCarlo, useStartWalkForward, useCompareRuns, useAnalyticsJob } from '../../hooks/useAnalytics'
import { CompareRuns } from './CompareRuns'
import { MonteCarloChart } from './MonteCarloChart'
import { ParamHeatmap } from './ParamHeatmap'
import { WalkForwardChart } from './WalkForwardChart'
import type { HeatmapResult, MonteCarloResult, WalkForwardResult } from '../../types'

interface Props {
  runId: number
  allRunIds?: number[] // for compare — IDs of all runs user has loaded
}

function Section({ title, children, defaultOpen = false }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className="border border-zinc-200 dark:border-zinc-700 rounded-lg overflow-hidden">
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center justify-between px-4 py-3 bg-zinc-50 dark:bg-zinc-800/50 text-sm font-medium text-zinc-900 dark:text-zinc-100 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors"
      >
        {title}
        {open ? <ChevronDown className="h-4 w-4 text-zinc-400" /> : <ChevronRight className="h-4 w-4 text-zinc-400" />}
      </button>
      {open && <div className="p-4">{children}</div>}
    </div>
  )
}

function JobSection<T>({
  title,
  onRun,
  isPending,
  jobId,
  renderResult,
  controls,
}: {
  title: string
  onRun: () => void
  isPending: boolean
  jobId: number | null
  renderResult: (data: T) => React.ReactNode
  controls?: React.ReactNode
}) {
  const job = useAnalyticsJob(jobId)
  const isRunning = job.data?.status === 'pending' || job.data?.status === 'running'
  const isComplete = job.data?.status === 'completed'
  const isFailed = job.data?.status === 'failed'

  return (
    <Section title={title}>
      <div className="space-y-3">
        <div className="flex items-center gap-3 flex-wrap">
          {controls}
          <button
            onClick={onRun}
            disabled={isPending || isRunning}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-violet-600 hover:bg-violet-700 disabled:bg-zinc-300 dark:disabled:bg-zinc-700 text-white rounded-lg transition-colors"
          >
            {(isPending || isRunning) && <Loader2 className="h-3 w-3 animate-spin" />}
            {isRunning ? 'Running...' : 'Run Analysis'}
          </button>
        </div>
        {isFailed && <p className="text-xs text-red-500">Analysis failed: {job.data?.error}</p>}
        {isComplete && job.data?.result && renderResult(job.data.result as T)}
      </div>
    </Section>
  )
}

export function AnalyticsTab({ runId, allRunIds = [] }: Props) {
  const [heatmapJobId, setHeatmapJobId] = useState<number | null>(null)
  const [mcJobId, setMcJobId] = useState<number | null>(null)
  const [wfJobId, setWfJobId] = useState<number | null>(null)
  const [xParam, setXParam] = useState('fast_period')
  const [yParam, setYParam] = useState('slow_period')
  const [gridSize, setGridSize] = useState(10)
  const [numSims, setNumSims] = useState(1000)
  const [numWindows, setNumWindows] = useState(5)

  const heatmapMut = useStartParamHeatmap()
  const mcMut = useStartMonteCarlo()
  const wfMut = useStartWalkForward()
  const compareIds = allRunIds.filter(id => id !== runId).slice(0, 4)
  const compareQuery = useCompareRuns(compareIds.length >= 1 ? [runId, ...compareIds] : [])

  const inputClass = "text-xs px-2 py-1 rounded border border-zinc-300 dark:border-zinc-600 bg-white dark:bg-zinc-800 text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-1 focus:ring-violet-500"

  return (
    <div className="space-y-3 py-4">
      {/* Param Heatmap */}
      <JobSection<HeatmapResult>
        title="Parameter Sensitivity Heatmap"
        isPending={heatmapMut.isPending}
        jobId={heatmapJobId}
        onRun={() => heatmapMut.mutate(
          { runId, xParam, yParam, gridSize },
          { onSuccess: (r) => setHeatmapJobId(r.job_id) }
        )}
        controls={
          <>
            <input className={inputClass} value={xParam} onChange={e => setXParam(e.target.value)} placeholder="X param" />
            <input className={inputClass} value={yParam} onChange={e => setYParam(e.target.value)} placeholder="Y param" />
            <input className={`${inputClass} w-16`} type="number" value={gridSize} onChange={e => setGridSize(Number(e.target.value))} placeholder="Grid" />
          </>
        }
        renderResult={(r) => <ParamHeatmap result={r} />}
      />

      {/* Monte Carlo */}
      <JobSection<MonteCarloResult>
        title="Monte Carlo Simulation"
        isPending={mcMut.isPending}
        jobId={mcJobId}
        onRun={() => mcMut.mutate(
          { runId, numSimulations: numSims },
          { onSuccess: (r) => setMcJobId(r.job_id) }
        )}
        controls={
          <input className={`${inputClass} w-24`} type="number" value={numSims} onChange={e => setNumSims(Number(e.target.value))} placeholder="# sims" />
        }
        renderResult={(r) => <MonteCarloChart result={r} />}
      />

      {/* Walk-Forward */}
      <JobSection<WalkForwardResult>
        title="Walk-Forward Analysis"
        isPending={wfMut.isPending}
        jobId={wfJobId}
        onRun={() => wfMut.mutate(
          { runId, windows: numWindows },
          { onSuccess: (r) => setWfJobId(r.job_id) }
        )}
        controls={
          <input className={`${inputClass} w-16`} type="number" value={numWindows} onChange={e => setNumWindows(Number(e.target.value))} placeholder="Windows" />
        }
        renderResult={(r) => <WalkForwardChart result={r} />}
      />

      {/* Compare Runs */}
      <Section title="Compare Runs">
        {compareIds.length === 0 ? (
          <p className="text-xs text-zinc-400">Load multiple backtest runs to compare them side-by-side.</p>
        ) : compareQuery.isLoading ? (
          <div className="flex items-center gap-2 text-xs text-zinc-400"><Loader2 className="h-3 w-3 animate-spin" /> Loading comparison...</div>
        ) : compareQuery.data ? (
          <CompareRuns result={compareQuery.data} />
        ) : null}
      </Section>
    </div>
  )
}
