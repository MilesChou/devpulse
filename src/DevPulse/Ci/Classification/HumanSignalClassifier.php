<?php

declare(strict_types=1);

namespace DevPulse\Ci\Classification;

final class HumanSignalClassifier
{
    /**
     * 把 build log 對 signal list 做字串比對，回傳第一個命中的 category。
     *
     * 沒命中回 null（caller 可視為 infra/flake 或留作 unknown）。
     * 比對順序依 signal 在 list 中的順序——caller 應把高優先順序的規則排前面。
     *
     * @param HumanSignal[] $signals
     */
    public function classify(string $log, array $signals): ?string
    {
        foreach ($signals as $signal) {
            if (str_contains($log, $signal->pattern)) {
                return $signal->category;
            }
        }

        return null;
    }
}
