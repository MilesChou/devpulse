<?php

declare(strict_types=1);

namespace Tests\Domain\Architecture;

use PHPat\Selector\Selector;
use PHPat\Test\Attributes\TestRule;
use PHPat\Test\Builder\Rule;
use PHPat\Test\PHPat;

/**
 * 強制 src/DevPulse 為純 domain layer：只可依賴自身 namespace + PHP 內建。
 *
 * 規則細節見 .claude/rules/domain-rules.md。
 */
final class DomainProductionPurityTest
{
    #[TestRule]
    public function productionOnlyDependsOnSelf(): Rule
    {
        return PHPat::rule()
            ->classes(Selector::inNamespace('DevPulse'))
            ->canOnly()
            ->dependOn()
            ->classes(Selector::inNamespace('DevPulse'))
            ->because('src/DevPulse is the pure domain layer; see .claude/rules/domain-rules.md');
    }
}
