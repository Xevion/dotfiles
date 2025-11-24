#!/usr/bin/env bun

// chezmoi update --init does not invoke the 'hooks.init.pre' hook, so we do it ourselves
const chezmoiArgs = process.env.CHEZMOI_ARGS || '';

if (chezmoiArgs.includes('init')) {
    // CHEZMOI_UPDATE is just a hint in case we need to know if we're updating
    const scriptDir = import.meta.dir;
    const initPreScript = `${scriptDir}/.init_pre.ts`;

    await Bun.spawn(['bun', initPreScript], {
        env: {
            ...process.env,
            CHEZMOI_UPDATE: '1'
        },
        stdout: 'inherit',
        stderr: 'inherit'
    });
}
