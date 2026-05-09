<?php

declare(strict_types=1);

namespace App\Support;

final class HumanSignalCategoryOptions
{
    /**
     * 後台下拉用的 human_signals.category 選項。
     *
     * VO（DevPulse\Ci\Classification\HumanSignal）並不限制 category 字串，
     * 此處只是給人輸入時的快捷清單；新增分類請改 config/devpulse.php。
     *
     * @return array<string, string>
     */
    public static function all(): array
    {
        $raw = config('devpulse.human_signal_categories');
        $categories = is_array($raw)
            ? array_values(array_filter($raw, 'is_string'))
            : [];

        return array_combine($categories, $categories);
    }
}
