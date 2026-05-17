<?php

declare(strict_types=1);

namespace Tests\Domain\Architecture;

use PHPat\Selector\Selector;
use PHPat\Test\Attributes\TestRule;
use PHPat\Test\Builder\Rule;
use PHPat\Test\PHPat;
use PHPUnit\Framework\TestCase;

/**
 * 強制 src/Tests（domain test）只可依賴 domain、PHPUnit 與 PHPat。
 *
 * 規則細節見 .claude/rules/domain-rules.md。
 */
final class DomainTestPurityTest
{
    #[TestRule]
    public function testOnlyDependsOnDomainAndPhpunit(): Rule
    {
        return PHPat::rule()
            ->classes(Selector::inNamespace('Tests\\Domain'))
            ->canOnly()
            ->dependOn()
            ->classes(
                Selector::inNamespace('DevPulse'),
                Selector::inNamespace('Tests\\Domain'),
                Selector::classname(TestCase::class),
                // architecture tests need phpat itself; treated as test infrastructure.
                Selector::inNamespace('PHPat'),
            )
            ->because('domain tests must not pull in framework code; see .claude/rules/domain-rules.md');
    }
}
