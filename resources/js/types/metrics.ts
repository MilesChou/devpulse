export interface ErrorRateSummary {
    rate: number | null;
    fails: number;
    denom: number;
}

export interface IterationSummary {
    ratio: number | null;
    builds: number;
    prs: number;
}

export interface LifecycleSummary {
    pickup_p90_seconds: number | null;
    approval_p90_seconds: number | null;
    merge_p90_seconds: number | null;
    pr_count: number;
}

export interface Thresholds {
    error_rate: number;
    iteration: number;
}

export interface DashboardProps {
    errorRate: ErrorRateSummary;
    iteration: IterationSummary;
    lifecycle: LifecycleSummary;
    thresholds: Thresholds;
    range: {
        from: string;
        to: string;
        days: number;
    };
}
