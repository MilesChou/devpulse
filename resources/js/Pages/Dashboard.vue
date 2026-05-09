<script setup lang="ts">
import { computed } from 'vue';
import { Head } from '@inertiajs/vue3';
import AppLayout from '@/Layouts/AppLayout.vue';
import type { DashboardProps } from '@/types/metrics';

const props = defineProps<DashboardProps>();

const SECONDS_PER_HOUR = 3600;
const SECONDS_PER_DAY = 86_400;

// ───────────────────────────────────────────── formatters
function fmtPercent(rate: number | null): string {
    if (rate === null) return '——';
    return `${(rate * 100).toFixed(1)}`;
}

function fmtRatio(ratio: number | null): string {
    if (ratio === null) return '——';
    if (ratio >= 100) return Math.round(ratio).toString();
    if (ratio >= 10) return ratio.toFixed(1);
    return ratio.toFixed(2);
}

function fmtDuration(seconds: number | null): { value: string; unit: string } {
    if (seconds === null) return { value: '——', unit: '' };
    if (seconds >= SECONDS_PER_DAY) {
        return { value: (seconds / SECONDS_PER_DAY).toFixed(1), unit: '天' };
    }
    if (seconds >= SECONDS_PER_HOUR) {
        return { value: (seconds / SECONDS_PER_HOUR).toFixed(1), unit: '小時' };
    }
    return { value: Math.round(seconds / 60).toString(), unit: '分' };
}

// ───────────────────────────────────────────── derived state
const errorPct = computed(() =>
    props.errorRate.rate === null ? null : props.errorRate.rate * 100,
);

const errorOver = computed(() =>
    props.errorRate.rate !== null &&
    props.errorRate.rate > props.thresholds.error_rate,
);

const iterOver = computed(() =>
    props.iteration.ratio !== null &&
    props.iteration.ratio > props.thresholds.iteration,
);

// 重推刻度尺：刻度 1~5，超過 5 釘到右端
const ITER_SCALE_MIN = 1;
const ITER_SCALE_MAX = 5;

function scaleToPercent(value: number): number {
    return ((value - ITER_SCALE_MIN) / (ITER_SCALE_MAX - ITER_SCALE_MIN)) * 100;
}

const iterPosition = computed(() => {
    if (props.iteration.ratio === null) return 0;
    return Math.min(100, scaleToPercent(props.iteration.ratio));
});

const errorThresholdPct = computed(() => props.thresholds.error_rate * 100);
const errorBarWidth = computed(() => {
    if (errorPct.value === null) return 0;
    return Math.min(100, errorPct.value);
});

const lifecycle = computed(() => ({
    pickup: fmtDuration(props.lifecycle.pickup_p90_seconds),
    approval: fmtDuration(props.lifecycle.approval_p90_seconds),
    merge: fmtDuration(props.lifecycle.merge_p90_seconds),
}));

const dateOpenSm = computed(() => props.range.from.slice(5));
const dateCloseSm = computed(() => props.range.to.slice(5));

function bigNumStyle(isOver: boolean) {
    return {
        fontFamily: 'var(--font-display)',
        fontWeight: 700,
        fontSize: 'clamp(72px, 18vw, 220px)',
        lineHeight: 0.85,
        fontVariationSettings: `'opsz' 144`,
        color: isOver ? 'var(--color-accent-warm)' : 'var(--color-paper)',
    };
}
</script>

<template>
    <Head title="儀表板" />
    <AppLayout>
        <div class="space-y-14 sm:space-y-20">
            <!-- INTRO ─────────────────────────────────────────────── -->
            <section class="dp-rise" style="animation-delay: 60ms;">
                <div class="grid grid-cols-12 gap-6 sm:gap-8 items-end">
                    <div class="col-span-12 md:col-span-8">
                        <div class="text-[11px] tracking-[0.3em]" style="color: var(--color-accent);">
                            ► 三項主要觀測
                        </div>
                        <h1
                            class="mt-4 leading-[0.92] tracking-[-0.04em]"
                            style="font-family: var(--font-display); font-weight: 300; font-size: clamp(40px, 7.5vw, 108px); font-variation-settings: 'opsz' 144, 'SOFT' 30;"
                        >
                            研發效能的
                            <span style="font-style: italic; font-weight: 600;">三項脈搏</span>
                        </h1>
                        <p class="mt-5 sm:mt-6 max-w-xl text-[13px] leading-relaxed" style="color: var(--color-paper-mute);">
                            錯誤率、重推次數、PR 一次完成度。三個數字、近 {{ props.range.days }} 天觀測窗、直接從 VCS 與 CI 原始資料計算。
                        </p>
                    </div>
                    <div class="col-span-12 md:col-span-4 grid grid-cols-2 gap-x-4 sm:gap-x-6 gap-y-3 text-[11px] tracking-[0.2em]" style="color: var(--color-paper-dim);">
                        <div>
                            <div>觀測窗</div>
                            <div class="mt-1 tabular-nums" style="color: var(--color-paper); font-size: 13px;">
                                {{ dateOpenSm }} → {{ dateCloseSm }}
                            </div>
                        </div>
                        <div>
                            <div>母體・PR</div>
                            <div class="mt-1 tabular-nums" style="color: var(--color-paper); font-size: 13px;">
                                {{ props.lifecycle.pr_count }}
                            </div>
                        </div>
                        <div>
                            <div>母體・Build</div>
                            <div class="mt-1 tabular-nums" style="color: var(--color-paper); font-size: 13px;">
                                {{ props.errorRate.denom }}
                            </div>
                        </div>
                        <div>
                            <div>計算基準</div>
                            <div class="mt-1 tabular-nums" style="color: var(--color-paper); font-size: 13px;">
                                VCS + CI 同源
                            </div>
                        </div>
                    </div>
                </div>

                <!-- 可調參數列 -->
                <div
                    class="mt-6 sm:mt-8 text-[10px] sm:text-[11px] tracking-[0.2em] py-3 px-4"
                    style="background: var(--color-ink-soft); border: 1px solid var(--color-ink-line); color: var(--color-paper-dim);"
                >
                    <div class="flex items-center gap-4 flex-wrap">
                        <span class="flex items-baseline gap-1.5">
                            <span>觀測窗</span>
                            <span class="tabular-nums" style="color: var(--color-paper); font-size: 12px;">
                                {{ props.range.days }}d
                            </span>
                        </span>
                        <span class="opacity-30">·</span>
                        <span class="flex items-baseline gap-1.5">
                            <span>錯誤閾值</span>
                            <span class="tabular-nums" style="color: var(--color-paper); font-size: 12px;">
                                {{ errorThresholdPct.toFixed(0) }}%
                            </span>
                        </span>
                        <span class="opacity-30">·</span>
                        <span class="flex items-baseline gap-1.5">
                            <span>重推閾值</span>
                            <span class="tabular-nums" style="color: var(--color-paper); font-size: 12px;">
                                {{ props.thresholds.iteration.toFixed(1) }}×
                            </span>
                        </span>
                    </div>
                    <div class="mt-2 text-[9px] sm:text-[10px] break-all" style="color: var(--color-paper-dim);">
                        以 query 覆寫：<code style="color: var(--color-accent);">?days=14&amp;error_threshold=0.25&amp;iteration_threshold=2.5</code>
                    </div>
                </div>
            </section>

            <!-- §1 ERROR RATE ───────────────────────────────────────── -->
            <section
                class="relative grid grid-cols-12 gap-6 sm:gap-8 dp-rise"
                style="animation-delay: 200ms;"
            >
                <aside class="col-span-12 md:col-span-3">
                    <div
                        class="text-[11px] tracking-[0.3em]"
                        :style="{ color: errorOver ? 'var(--color-accent-warm)' : 'var(--color-accent)' }"
                    >
                        § 1
                    </div>
                    <h2
                        class="mt-3 leading-[0.95] tracking-[-0.02em] dp-section-h2"
                    >
                        <span style="font-style: italic; font-weight: 600;">錯誤率，</span><br />
                        紅燈與綠燈
                    </h2>
                    <p class="mt-4 text-[13px] leading-relaxed" style="color: var(--color-paper-mute);">
                        失敗 build ÷（總 build − canceled），排除 post-merge 與 deploy。閾值 {{ errorThresholdPct.toFixed(0) }}%。
                    </p>
                </aside>

                <div class="col-span-12 md:col-span-9">
                    <div
                        class="relative p-5 sm:p-8 pt-8 sm:pt-9"
                        :style="{
                            background: errorOver ? 'rgba(255,91,31,0.06)' : 'var(--color-ink-soft)',
                            border: '1px solid ' + (errorOver ? '#ff5b1f80' : 'var(--color-ink-line)'),
                        }"
                    >
                        <div
                            class="absolute -top-3 left-4 sm:left-6 px-2 py-0.5 text-[10px] tracking-[0.3em]"
                            :style="{
                                background: 'var(--color-ink)',
                                color: errorOver ? 'var(--color-accent-warm)' : 'var(--color-paper-dim)',
                                border: '1px solid ' + (errorOver ? '#ff5b1f80' : 'var(--color-ink-line)'),
                            }"
                        >
                            指標一・錯誤率
                            <span v-if="errorOver" class="ml-2" style="color: var(--color-accent-warm);">⚠ 超標</span>
                        </div>

                        <div class="flex items-baseline gap-4 flex-wrap">
                            <div
                                class="tabular-nums tracking-[-0.05em] flex items-baseline"
                                :style="bigNumStyle(errorOver)"
                            >
                                {{ fmtPercent(props.errorRate.rate) }}
                                <span class="ml-2" style="font-family: var(--font-mono); font-weight: 400; font-size: 0.18em; color: var(--color-paper-dim);">%</span>
                            </div>
                            <div class="text-[10px] sm:text-[11px] tracking-[0.2em] sm:tracking-[0.25em] tabular-nums sm:ml-auto" style="color: var(--color-paper-dim);">
                                {{ props.errorRate.fails }} 失敗 / {{ props.errorRate.denom }} 計入
                            </div>
                        </div>

                        <!-- progress meter -->
                        <div class="mt-8 relative h-2.5" style="background: var(--color-ink); border: 1px solid var(--color-ink-line);">
                            <!-- 閾值垂直線 -->
                            <div
                                class="absolute top-[-6px] bottom-[-6px] w-px"
                                :style="{
                                    left: `${errorThresholdPct}%`,
                                    background: 'var(--color-accent-warm)',
                                }"
                            />
                            <div
                                class="absolute -top-4 text-[9px] tracking-[0.2em] -translate-x-1/2"
                                :style="{ left: `${errorThresholdPct}%`, color: 'var(--color-accent-warm)' }"
                            >
                                閾值 {{ errorThresholdPct.toFixed(0) }}%
                            </div>
                            <!-- 實際值 -->
                            <div
                                class="absolute top-0 bottom-0 left-0 transition-all"
                                :style="{
                                    width: `${errorBarWidth}%`,
                                    background: errorOver ? 'var(--color-accent-warm)' : 'var(--color-accent)',
                                }"
                            />
                            <!-- 0 / 50 / 100 刻度 -->
                            <div class="absolute left-0 top-full mt-1 text-[9px] tracking-[0.2em]" style="color: var(--color-paper-dim);">0%</div>
                            <div class="absolute left-1/2 top-full mt-1 -translate-x-1/2 text-[9px] tracking-[0.2em]" style="color: var(--color-paper-dim);">50%</div>
                            <div class="absolute right-0 top-full mt-1 text-[9px] tracking-[0.2em]" style="color: var(--color-paper-dim);">100%</div>
                        </div>
                    </div>
                </div>
            </section>

            <!-- §2 ITERATION ───────────────────────────────────────── -->
            <section
                class="relative grid grid-cols-12 gap-6 sm:gap-8 dp-rise"
                style="animation-delay: 320ms;"
            >
                <aside class="col-span-12 md:col-span-3">
                    <div
                        class="text-[11px] tracking-[0.3em]"
                        :style="{ color: iterOver ? 'var(--color-accent-warm)' : 'var(--color-accent)' }"
                    >
                        § 2
                    </div>
                    <h2
                        class="mt-3 leading-[0.95] tracking-[-0.02em] dp-section-h2"
                    >
                        <span style="font-style: italic; font-weight: 600;">重推次數，</span><br />
                        每張 PR 推幾回
                    </h2>
                    <p class="mt-4 text-[13px] leading-relaxed" style="color: var(--color-paper-mute);">
                        Build 數 ÷ PR 數。理想值是 1.00；高於 {{ props.thresholds.iteration.toFixed(1) }} 視為過度迭代。
                    </p>
                </aside>

                <div class="col-span-12 md:col-span-9">
                    <div
                        class="relative p-5 sm:p-8 pt-8 sm:pt-9"
                        :style="{
                            background: iterOver ? 'rgba(255,91,31,0.06)' : 'var(--color-ink-soft)',
                            border: '1px solid ' + (iterOver ? '#ff5b1f80' : 'var(--color-ink-line)'),
                        }"
                    >
                        <div
                            class="absolute -top-3 left-4 sm:left-6 px-2 py-0.5 text-[10px] tracking-[0.3em]"
                            :style="{
                                background: 'var(--color-ink)',
                                color: iterOver ? 'var(--color-accent-warm)' : 'var(--color-paper-dim)',
                                border: '1px solid ' + (iterOver ? '#ff5b1f80' : 'var(--color-ink-line)'),
                            }"
                        >
                            指標二・重推
                            <span v-if="iterOver" class="ml-2" style="color: var(--color-accent-warm);">⚠ 超標</span>
                        </div>

                        <div class="flex items-baseline gap-4 flex-wrap">
                            <div
                                class="tabular-nums tracking-[-0.05em] flex items-baseline"
                                :style="bigNumStyle(iterOver)"
                            >
                                {{ fmtRatio(props.iteration.ratio) }}
                                <span class="ml-2" style="font-family: var(--font-mono); font-weight: 400; font-size: 0.18em; color: var(--color-paper-dim);">×</span>
                            </div>
                            <div class="text-[10px] sm:text-[11px] tracking-[0.2em] sm:tracking-[0.25em] tabular-nums sm:ml-auto" style="color: var(--color-paper-dim);">
                                {{ props.iteration.builds }} builds / {{ props.iteration.prs }} PRs
                            </div>
                        </div>

                        <!-- 刻度尺：1~5，超過 5 顯示為右端外 -->
                        <div class="mt-7 relative">
                            <div class="relative h-px" style="background: var(--color-ink-line);">
                                <!-- 大刻度 1,2,3,4,5 -->
                                <template v-for="tick in [1,2,3,4,5]" :key="tick">
                                    <div
                                        class="absolute top-0 w-px"
                                        :style="{
                                            left: `${scaleToPercent(tick)}%`,
                                            height: tick === 1 || tick === 3 ? '14px' : '8px',
                                            transform: 'translateY(-3px)',
                                            background: tick === 3
                                                ? 'var(--color-accent-warm)'
                                                : tick === 1
                                                    ? 'var(--color-accent)'
                                                    : 'var(--color-paper-dim)',
                                        }"
                                    />
                                </template>
                                <!-- 1 是理想（左端） -->
                                <div
                                    class="absolute top-3 text-[9px] tracking-[0.2em]"
                                    style="left: 0; color: var(--color-accent);"
                                >
                                    理想 1.0
                                </div>
                                <!-- 閾值標籤 -->
                                <div
                                    class="absolute top-3 -translate-x-1/2 text-[9px] tracking-[0.2em]"
                                    :style="{ left: `${scaleToPercent(props.thresholds.iteration)}%`, color: 'var(--color-accent-warm)' }"
                                >
                                    閾值 {{ props.thresholds.iteration.toFixed(1) }}
                                </div>
                                <!-- 1,5+ 端點標籤 -->
                                <div class="absolute -top-5 left-0 text-[9px] tracking-[0.2em]" style="color: var(--color-paper-dim);">
                                    1
                                </div>
                                <div class="absolute -top-5 right-0 text-[9px] tracking-[0.2em]" style="color: var(--color-paper-dim);">
                                    5+
                                </div>

                                <!-- 當前位置游標 -->
                                <div
                                    v-if="props.iteration.ratio !== null"
                                    class="absolute -translate-x-1/2"
                                    :style="{ left: `${iterPosition}%`, top: '-22px' }"
                                >
                                    <div
                                        class="w-3 h-3 rotate-45"
                                        :style="{
                                            background: iterOver ? 'var(--color-accent-warm)' : 'var(--color-accent)',
                                            boxShadow: iterOver
                                                ? '0 0 16px rgba(255,91,31,0.6)'
                                                : '0 0 16px rgba(196,240,0,0.4)',
                                        }"
                                    />
                                </div>
                            </div>
                            <div class="mt-10 text-[11px] tracking-[0.2em]" style="color: var(--color-paper-mute);">
                                <span v-if="props.iteration.ratio === null">無資料可評估。</span>
                                <span v-else-if="!iterOver">在閾值內・每張 PR 平均推 <span style="color: var(--color-paper);">{{ fmtRatio(props.iteration.ratio) }}</span> 次 build。</span>
                                <span v-else style="color: var(--color-accent-warm);">
                                    超出閾值・PR 反覆推送的成本正在累積。
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <!-- §3 LIFECYCLE p90 ──────────────────────────────────── -->
            <section
                class="relative grid grid-cols-12 gap-6 sm:gap-8 dp-rise"
                style="animation-delay: 440ms;"
            >
                <aside class="col-span-12 md:col-span-3">
                    <div class="text-[11px] tracking-[0.3em]" style="color: var(--color-accent);">
                        § 3
                    </div>
                    <h2
                        class="mt-3 leading-[0.95] tracking-[-0.02em] dp-section-h2"
                    >
                        PR 一次完成度，
                        <br />
                        <span style="font-style: italic; font-weight: 600;">三個 p90</span>
                    </h2>
                    <p class="mt-4 text-[13px] leading-relaxed" style="color: var(--color-paper-mute);">
                        從待審到合併，三段時長的 90th percentile。
                        <br /><br />
                        Pickup 量「是否被看見」、Approval 量「是否被認可」、Merge 量「是否能落地」。
                    </p>
                </aside>

                <div class="col-span-12 md:col-span-9">
                    <div
                        class="relative grid grid-cols-1 sm:grid-cols-3"
                        style="background: var(--color-ink-soft); border: 1px solid var(--color-ink-line);"
                    >
                        <div class="absolute -top-3 left-4 sm:left-6 px-2 py-0.5 text-[10px] tracking-[0.3em]"
                             style="background: var(--color-ink); color: var(--color-paper-dim); border: 1px solid var(--color-ink-line);">
                            指標三・p90 lifecycle
                        </div>

                        <div class="p-5 sm:p-8 pt-10 sm:pt-12 sm:border-r border-b sm:border-b-0" style="border-color: var(--color-ink-line);">
                            <div class="text-[10px] tracking-[0.3em] flex items-center gap-2" style="color: var(--color-paper-dim);">
                                <span class="w-1.5 h-1.5 rotate-45" style="background: var(--color-accent);"></span>
                                Pickup
                            </div>
                            <div class="mt-4 tabular-nums tracking-[-0.04em] flex items-baseline gap-2 dp-stat-num-lg">
                                {{ lifecycle.pickup.value }}
                                <span class="text-[14px] tracking-[0.2em]" style="font-family: var(--font-mono); font-weight: 400; color: var(--color-paper-dim);">
                                    {{ lifecycle.pickup.unit }}
                                </span>
                            </div>
                            <div class="mt-3 text-[11px] leading-relaxed" style="color: var(--color-paper-mute);">
                                ready_at → 第一筆 review 的 p90
                            </div>
                        </div>

                        <div class="p-5 sm:p-8 sm:pt-12 sm:border-r border-b sm:border-b-0" style="border-color: var(--color-ink-line);">
                            <div class="text-[10px] tracking-[0.3em] flex items-center gap-2" style="color: var(--color-paper-dim);">
                                <span class="w-1.5 h-1.5 rotate-45" style="background: var(--color-accent-warm);"></span>
                                Approval
                            </div>
                            <div class="mt-4 tabular-nums tracking-[-0.04em] flex items-baseline gap-2 dp-stat-num-lg">
                                {{ lifecycle.approval.value }}
                                <span class="text-[14px] tracking-[0.2em]" style="font-family: var(--font-mono); font-weight: 400; color: var(--color-paper-dim);">
                                    {{ lifecycle.approval.unit }}
                                </span>
                            </div>
                            <div class="mt-3 text-[11px] leading-relaxed" style="color: var(--color-paper-mute);">
                                ready_at → 第一筆 APPROVED 的 p90
                            </div>
                        </div>

                        <div class="p-5 sm:p-8 sm:pt-12" style="border-color: var(--color-ink-line);">
                            <div class="text-[10px] tracking-[0.3em] flex items-center gap-2" style="color: var(--color-paper-dim);">
                                <span class="w-1.5 h-1.5 rotate-45" style="background: var(--color-accent-cool);"></span>
                                Merge
                            </div>
                            <div class="mt-4 tabular-nums tracking-[-0.04em] flex items-baseline gap-2 dp-stat-num-lg">
                                {{ lifecycle.merge.value }}
                                <span class="text-[14px] tracking-[0.2em]" style="font-family: var(--font-mono); font-weight: 400; color: var(--color-paper-dim);">
                                    {{ lifecycle.merge.unit }}
                                </span>
                            </div>
                            <div class="mt-3 text-[11px] leading-relaxed" style="color: var(--color-paper-mute);">
                                第一筆 APPROVED → merged 的 p90
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <!-- COLOPHON ─────────────────────────────────────────── -->
            <section
                class="border-t pt-8 sm:pt-10 grid grid-cols-12 gap-6 dp-rise"
                style="border-color: var(--color-ink-line); animation-delay: 560ms;"
            >
                <div class="col-span-12 md:col-span-4">
                    <div class="text-[11px] tracking-[0.3em]" style="color: var(--color-paper-dim);">版式</div>
                    <p class="mt-3 leading-snug"
                       style="font-family: var(--font-display); font-weight: 400; font-size: 22px; color: var(--color-paper);">
                        以 <em>Fraunces</em> 與 <em>JetBrains Mono</em> 排印。<br />
                        由 Inertia 編排、Laravel 印行。
                    </p>
                </div>
                <div class="col-span-12 md:col-span-4">
                    <div class="text-[11px] tracking-[0.3em]" style="color: var(--color-paper-dim);">計算基準</div>
                    <p class="mt-3 text-[13px] leading-relaxed" style="color: var(--color-paper-mute);">
                        錯誤率排除 <span style="color: var(--color-paper);">is_post_merge</span> 與 <span style="color: var(--color-paper);">is_deploy_event</span>，分母不含 <span style="color: var(--color-paper);">CANCELED</span>。重推僅計 <span style="color: var(--color-paper);">is_pull_request</span> 且 <span style="color: var(--color-paper);">pr_number</span> 不為空的 build。p90 母體為 <span style="color: var(--color-paper);">pr_created_at</span> 在區間內、<span style="color: var(--color-paper);">ready_at</span> 不為空的 PR。
                    </p>
                </div>
                <div class="col-span-12 md:col-span-4">
                    <div class="text-[11px] tracking-[0.3em]" style="color: var(--color-paper-dim);">下一篇</div>
                    <ul class="mt-3 space-y-1.5 text-[13px]" style="color: var(--color-paper-mute);">
                        <li>— 群組維度的拆解</li>
                        <li>— 趨勢圖（每日線）</li>
                        <li>— 設定面板（取代 query string）</li>
                        <li>— OAuth 登入與 email 摘要</li>
                    </ul>
                </div>
            </section>
        </div>
    </AppLayout>
</template>
