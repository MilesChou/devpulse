<?php

declare(strict_types=1);

namespace DevPulse\Vcs;

interface PullRequestFactory
{
    /**
     * Build a PullRequest from a platform-specific raw payload.
     *
     * Each implementation owns the payload shape for its own platform (REST/GraphQL/etc.).
     *
     * @param array<string, mixed> $raw
     */
    public function fromRaw(array $raw, string $repoId, PullRequestId $id): PullRequest;
}
