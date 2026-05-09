<script setup lang="ts">
import { Link } from '@inertiajs/vue3';
import { computed, onMounted, onUnmounted, ref } from 'vue';

const now = ref(new Date());
const session = ref(generateSession());

function generateSession(): string {
    return Math.random().toString(36).slice(2, 8).toUpperCase();
}

function pad(n: number): string {
    return n.toString().padStart(2, '0');
}

const clock = computed(() => {
    const d = now.value;
    return `${d.getUTCFullYear()}.${pad(d.getUTCMonth() + 1)}.${pad(d.getUTCDate())} · ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())} UTC`;
});

const issueNo = computed(() => {
    const d = now.value;
    const year = d.getUTCFullYear();
    const startOfYear = Date.UTC(year, 0, 1);
    const dayOfYear = Math.floor((+d - startOfYear) / 86_400_000) + 1;
    return `№ ${year}.${dayOfYear.toString().padStart(3, '0')}`;
});

let timer: ReturnType<typeof setInterval>;

onMounted(() => {
    timer = setInterval(() => { now.value = new Date(); }, 60_000);
});

onUnmounted(() => {
    clearInterval(timer);
});

const tickerItems = [
    'PR 流程・正常',
    'CI 吞吐量・穩定',
    '聚合排程・最近一輪 OK',
    '資料窗・近 30 天',
    '每日刊號・發行中',
    '無事故回報',
];
</script>

<template>
    <div class="relative min-h-screen flex flex-col text-paper" style="color: var(--color-paper);">
        <!-- 頂部跑馬燈・狀態帶 -->
        <div
            class="relative z-10 overflow-hidden border-b text-[10px] uppercase tracking-[0.4em]"
            style="border-color: var(--color-ink-line); background: var(--color-ink-soft);"
        >
            <div class="flex whitespace-nowrap py-2 dp-marquee" style="width: max-content;">
                <span
                    v-for="block in 2"
                    :key="block"
                    class="flex items-center gap-10 px-6"
                    style="color: var(--color-paper-dim);"
                >
                    <span
                        v-for="item in tickerItems"
                        :key="`${block}-${item}`"
                        class="flex items-center gap-3"
                    >
                        <span
                            class="inline-block w-1.5 h-1.5 rounded-full"
                            style="background: var(--color-accent);"
                        />
                        {{ item }}
                    </span>
                </span>
            </div>
        </div>

        <!-- 刊頭 -->
        <header class="relative z-10 border-b" style="border-color: var(--color-ink-line);">
            <div class="max-w-[1400px] mx-auto px-4 sm:px-8 pt-6 sm:pt-8 pb-5 sm:pb-6">
                <div class="flex items-end justify-between gap-4 flex-wrap">
                    <div class="flex flex-col gap-1">
                        <Link
                            href="/dashboard"
                            class="leading-none tracking-[-0.04em] inline-flex items-baseline gap-2 hover:opacity-90"
                            style="font-family: var(--font-display); font-weight: 800; font-size: clamp(36px, 10vw, 96px); font-variation-settings: 'opsz' 144, 'SOFT' 0, 'WONK' 1;"
                        >
                            <span class="dp-glitch">devpulse</span>
                            <span
                                class="text-[9px] sm:text-[10px] uppercase tracking-[0.4em] translate-y-[-1.4em] inline-block"
                                style="font-family: var(--font-mono); color: var(--color-accent);"
                            >※ 觀測誌</span>
                        </Link>
                        <p
                            class="text-[10px] sm:text-[11px] tracking-[0.2em] mt-1 sm:mt-2"
                            style="color: var(--color-paper-dim);"
                        >
                            一份內部研發效能年鑑 ─ 概念驗證版
                        </p>
                    </div>

                    <div class="hidden sm:flex items-stretch text-[10px] uppercase tracking-[0.3em]"
                         style="color: var(--color-paper-mute);"
                    >
                        <div class="px-4 py-2 border" style="border-color: var(--color-ink-line);">
                            <div style="color: var(--color-paper-dim);">刊號</div>
                            <div class="text-base mt-1" style="font-family: var(--font-display); font-weight: 600; letter-spacing: -0.02em; color: var(--color-paper);">{{ issueNo }}</div>
                        </div>
                        <div class="px-4 py-2 border-t border-r border-b" style="border-color: var(--color-ink-line);">
                            <div style="color: var(--color-paper-dim);">會話</div>
                            <div class="text-base mt-1" style="font-family: var(--font-mono); color: var(--color-accent);">{{ session }}</div>
                        </div>
                        <div class="px-4 py-2 border-t border-r border-b" style="border-color: var(--color-ink-line);">
                            <div style="color: var(--color-paper-dim);">時戳</div>
                            <div class="text-base mt-1 tabular-nums">{{ clock }}</div>
                        </div>
                    </div>
                    <div class="flex sm:hidden items-center gap-3 text-[9px] tracking-[0.15em]"
                         style="color: var(--color-paper-dim);"
                    >
                        <span style="font-family: var(--font-display); font-weight: 600; color: var(--color-paper);">{{ issueNo }}</span>
                        <span class="opacity-30">·</span>
                        <span class="tabular-nums">{{ clock }}</span>
                    </div>
                </div>

                <!-- 副欄・分隔線 + 導覽 -->
                <div class="mt-5 sm:mt-7 flex items-center justify-between gap-4 flex-wrap">
                    <nav class="flex items-center gap-5 sm:gap-8 text-[10px] sm:text-[11px] tracking-[0.2em]">
                        <Link
                            href="/dashboard"
                            class="relative pb-2 inline-flex items-center gap-2"
                            style="color: var(--color-paper);"
                        >
                            <span
                                class="inline-block w-2 h-2 rotate-45"
                                style="background: var(--color-accent);"
                            />
                            儀表板
                            <span
                                class="absolute left-0 right-0 -bottom-px h-px"
                                style="background: var(--color-paper);"
                            />
                        </Link>
                        <span style="color: var(--color-paper-dim);">報表 —</span>
                        <span style="color: var(--color-paper-dim);">設定 —</span>
                        <span class="hidden sm:inline" style="color: var(--color-paper-dim);">封存 —</span>
                    </nav>
                    <div
                        class="text-[10px] tracking-[0.3em] dp-cursor"
                        style="color: var(--color-paper-dim);"
                    >
                        即時連線中
                    </div>
                </div>
            </div>
        </header>

        <!-- 主內容 -->
        <main class="relative z-10 flex-1 max-w-[1400px] w-full mx-auto px-4 sm:px-8 py-10 sm:py-14">
            <slot />
        </main>

        <!-- 頁尾 -->
        <footer class="relative z-10 border-t mt-10" style="border-color: var(--color-ink-line);">
            <div class="max-w-[1400px] mx-auto px-4 sm:px-8 py-6 flex items-center justify-between gap-4 flex-wrap text-[10px] tracking-[0.3em]" style="color: var(--color-paper-dim);">
                <div>
                    devpulse 觀測誌 ■ 內部流通 ■ 印於電報線上
                </div>
                <div class="flex items-center gap-6">
                    <span>建置 {{ session.toLowerCase() }}</span>
                    <span>第 01 版</span>
                </div>
            </div>
        </footer>
    </div>
</template>
