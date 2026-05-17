<?php

declare(strict_types=1);

namespace Tests\Feature;

use Tests\TestCase;

class ExampleTest extends TestCase
{
    public function testApplicationBoots(): void
    {
        $this->assertTrue($this->app->bound('config'));
    }
}
