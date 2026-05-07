<?php

declare(strict_types=1);

namespace App\Reporting\Renderer;

use App\Aggregation\Dto\FailedBuildItem;
use Illuminate\Support\Collection;

final class FailedBuildListRenderer
{
    /**
     * @param Collection<int, FailedBuildItem> $items
     */
    public function render(Collection $items): string
    {
        if ($items->isEmpty()) {
            return "## 失敗 Build 清單\n\n（本月無失敗 build）\n";
        }

        $lines = [
            '## 失敗 Build 清單',
            '',
            '| 日期 | Repo | Author | PR | Commit | Status |',
            '| --- | --- | --- | --- | --- | --- |',
        ];

        foreach ($items as $item) {
            $repo = (string)$item->repoFullName;
            $prCell = $item->prNumber !== null
                ? sprintf('[#%d](https://github.com/%s/pull/%d)', $item->prNumber->value, $repo, $item->prNumber->value)
                : '—';
            $shortSha = substr((string)$item->commitSha, 0, 7);
            $commitCell = sprintf('[%s](https://github.com/%s/commit/%s)', $shortSha, $repo, $item->commitSha);

            $lines[] = sprintf(
                '| %s | %s | %s | %s | %s | %s |',
                $item->startedAt->format('Y-m-d H:i'),
                $repo,
                $item->authorAccount !== null ? (string)$item->authorAccount : '—',
                $prCell,
                $commitCell,
                $item->status,
            );
        }
        $lines[] = '';

        return implode("\n", $lines);
    }
}
