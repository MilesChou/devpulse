<?php

declare(strict_types=1);

namespace App\Infrastructure\Vcs\GitHub;

use App\Domain\Shared\RepoFullName;
use InvalidArgumentException;

/**
 * 一次查多筆 commit 的 author GitHub login（GraphQL alias 模式）。
 *
 * GitHub GraphQL 不支援 oid 作為 variable，所以 sha 直接 inline 進 query。
 * sha 是 40 字元 hex（已驗證安全），不會有 injection 風險。
 *
 * 仿 Python prototype `gh_commits_authors_bulk`，每批最多 80 筆 alias。
 */
class GetCommitAuthorsBulkQuery extends GraphQLRequest
{
    public const MAX_BATCH = 80;

    /**
     * @param list<string> $shas commit SHA 清單（最多 MAX_BATCH 筆）
     */
    public function __construct(
        private readonly RepoFullName $repoFullName,
        private readonly array $shas,
    ) {
        if (count($this->shas) > self::MAX_BATCH) {
            throw new InvalidArgumentException(
                'shas count exceeds GraphQL alias batch limit (' . self::MAX_BATCH . ')',
            );
        }
        foreach ($this->shas as $sha) {
            if (!preg_match('/^[a-f0-9]{7,40}$/', $sha)) {
                throw new InvalidArgumentException("invalid sha format: {$sha}");
            }
        }
    }

    protected function graphqlQuery(): string
    {
        $body = '';
        foreach ($this->shas as $i => $sha) {
            $body .= sprintf(
                ' c%d: object(oid: "%s") { ... on Commit { oid author { user { login } } } }',
                $i,
                $sha,
            );
        }

        return <<<GRAPHQL
            query(\$owner: String!, \$name: String!) {
              repository(owner: \$owner, name: \$name) {{$body}
              }
            }
            GRAPHQL;
    }

    /**
     * @return array<string, mixed>
     */
    protected function graphqlVariables(): array
    {
        return [
            'owner' => $this->repoFullName->owner,
            'name' => $this->repoFullName->name,
        ];
    }
}
