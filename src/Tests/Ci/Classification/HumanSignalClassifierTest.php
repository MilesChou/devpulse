<?php

declare(strict_types=1);

namespace Tests\Domain\Ci\Classification;

use DevPulse\Ci\Classification\HumanSignal;
use DevPulse\Ci\Classification\HumanSignalClassifier;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class HumanSignalClassifierTest extends TestCase
{
    public function testReturnsCategoryWhenLogContainsPattern(): void
    {
        $classifier = new HumanSignalClassifier();
        $signals = [
            new HumanSignal(category: 'lint', pattern: 'PHPCS:'),
        ];

        $log = "Running PHPCS...\nPHPCS: ERROR Line too long\nDone.";
        $this->assertSame('lint', $classifier->classify($log, $signals));
    }

    public function testReturnsNullWhenNoSignalMatches(): void
    {
        $classifier = new HumanSignalClassifier();
        $signals = [
            new HumanSignal(category: 'lint', pattern: 'PHPCS:'),
            new HumanSignal(category: 'test', pattern: 'PHPUnit failed'),
        ];

        $log = "Connection timed out to docker registry";
        $this->assertNull($classifier->classify($log, $signals));
    }

    public function testReturnsFirstMatchingCategoryByOrder(): void
    {
        $classifier = new HumanSignalClassifier();
        // 規則順序決定優先順序：lint 先匹到就回 lint，不會繼續找 test
        $signals = [
            new HumanSignal(category: 'lint', pattern: 'ERROR'),
            new HumanSignal(category: 'test', pattern: 'ERROR'),
        ];

        $log = "Build failed: ERROR";
        $this->assertSame('lint', $classifier->classify($log, $signals));
    }

    public function testReturnsNullWhenSignalListEmpty(): void
    {
        $classifier = new HumanSignalClassifier();
        $this->assertNull($classifier->classify('any log', []));
    }

    public function testHumanSignalThrowsWhenCategoryEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('category');
        new HumanSignal(category: '', pattern: 'PHPCS:');
    }

    public function testHumanSignalThrowsWhenPatternEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('pattern');
        new HumanSignal(category: 'lint', pattern: '');
    }

    public function testFromArrayCreatesValidVo(): void
    {
        $signal = HumanSignal::fromArray(['category' => 'test', 'pattern' => 'PHPUnit']);
        $this->assertSame('test', $signal->category);
        $this->assertSame('PHPUnit', $signal->pattern);
    }
}
