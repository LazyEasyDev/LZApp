(async () => {
  const reference = document.getElementById('api-reference');
  const loader = document.getElementById('lzapp-scalar-loader');
  if (!reference || !loader) return;

  function pruneUnusedComponents(schema) {
    const components = schema.components;
    if (!components || typeof components !== 'object' || Array.isArray(components)) return;

    const retained = new Map();
    const visited = new Set();
    const pending = [Object.fromEntries(Object.entries(schema)
      .filter(([name]) => name !== 'components'))];
    let unresolvedReference = false;

    function retain(group, name) {
      const definitions = components[group];
      if (!definitions || typeof definitions !== 'object' || !Object.hasOwn(definitions, name)) return;
      if (!retained.has(group)) retained.set(group, new Set());
      if (retained.get(group).has(name)) return;
      retained.get(group).add(name);
      pending.push(definitions[name]);
    }

    function followReference(reference, allowSchemaName = false) {
      if (typeof reference !== 'string') return;
      if (allowSchemaName && components.schemas && Object.hasOwn(components.schemas, reference)) {
        retain('schemas', reference);
        return;
      }
      if (!reference.startsWith('#/')) {
        unresolvedReference = true;
        return;
      }
      let parts;
      try {
        parts = decodeURIComponent(reference.slice(1)).slice(1).split('/')
          .map(part => part.replace(/~1/g, '/').replace(/~0/g, '~'));
      } catch {
        unresolvedReference = true;
        return;
      }
      let target = schema;
      for (const part of parts) {
        if (!target || typeof target !== 'object' || !Object.hasOwn(target, part)) {
          unresolvedReference = true;
          return;
        }
        target = target[part];
      }
      if (parts[0] === 'components') {
        if (parts.length < 3) {
          unresolvedReference = true;
          return;
        }
        retain(parts[1], parts[2]);
      }
      pending.push(target);
    }

    while (pending.length > 0) {
      const value = pending.pop();
      if (!value || typeof value !== 'object' || visited.has(value)) continue;
      visited.add(value);
      followReference(value.$ref);
      followReference(value.$dynamicRef);
      if (value.discriminator?.mapping && typeof value.discriminator.mapping === 'object') {
        for (const reference of Object.values(value.discriminator.mapping)) {
          followReference(reference, true);
        }
      }
      if (Array.isArray(value.security)) {
        for (const requirement of value.security) {
          if (!requirement || typeof requirement !== 'object') continue;
          for (const name of Object.keys(requirement)) retain('securitySchemes', name);
        }
      }
      for (const child of Object.values(value)) pending.push(child);
    }

    if (unresolvedReference) return;
    for (const group of ['schemas', 'responses', 'parameters', 'examples', 'requestBodies',
      'headers', 'securitySchemes', 'links', 'callbacks', 'pathItems']) {
      const definitions = components[group];
      if (!definitions || typeof definitions !== 'object' || Array.isArray(definitions)) continue;
      for (const name of Object.keys(definitions)) {
        if (!retained.get(group)?.has(name)) delete definitions[name];
      }
      if (Object.keys(definitions).length === 0) delete components[group];
    }
  }

  let configuration;
  try {
    configuration = JSON.parse(reference.dataset.configuration || '{}');
  } catch {
    configuration = {};
  }
  configuration.persistAuth = false;

  const schemaURL = reference.dataset.url || '/openapi.json';
  reference.removeAttribute('data-url');
  delete configuration.url;
  delete configuration.sources;
  configuration.content = {
    openapi: '3.1.0',
    info: { title: 'API Documentation', version: '1.0.0' },
    paths: {}
  };
  let allowedOperations = new Set();
  let showAll = false;

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 3000);
  try {
    const response = await fetch('/docs_token', {
      method: 'POST',
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      headers: { 'X-LZApp-Docs': '1', Accept: 'application/json' },
      signal: controller.signal
    });
    if (response.ok && response.headers.get('content-type')?.includes('application/json')) {
      const result = await response.json();
      if (result && typeof result === 'object' && !Array.isArray(result)) {
        showAll = result.showAll === true;
        if (Array.isArray(result.allowedOperations)) {
          allowedOperations = new Set(result.allowedOperations.filter(operation =>
            typeof operation === 'string' &&
            /^(GET|PUT|POST|DELETE|OPTIONS|HEAD|PATCH|TRACE) \/\S*$/.test(operation)
          ));
        }
        if (typeof result.token === 'string' && result.token.length <= 8192 &&
            /^[A-Za-z0-9._~+/-]+=*$/.test(result.token)) {
          configuration.authentication = {
            ...configuration.authentication,
            preferredSecurityScheme: 'bearerAuth',
            securitySchemes: {
              ...configuration.authentication?.securitySchemes,
              bearerAuth: { token: result.token }
            }
          };
        }
      }
    }
  } catch {
  } finally {
    clearTimeout(timeout);
  }

  const schemaController = new AbortController();
  const schemaTimeout = setTimeout(() => schemaController.abort(), 5000);
  try {
    const response = await fetch(schemaURL, {
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      headers: { Accept: 'application/json' },
      signal: schemaController.signal
    });
    if (response.ok) {
      const document = await response.json();
      if (document && typeof document === 'object' && !Array.isArray(document) &&
          typeof document.openapi === 'string' && document.info) {
        const content = structuredClone(document);
        if (!showAll) {
          const methods = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'];
          const visiblePaths = Object.create(null);
          for (const [path, pathItem] of Object.entries(content.paths || {})) {
            if (!pathItem || typeof pathItem !== 'object' || Array.isArray(pathItem)) continue;
            let visible = false;
            for (const method of methods) {
              if (Object.hasOwn(pathItem, method) &&
                  allowedOperations.has(`${method.toUpperCase()} ${path}`)) {
                visible = true;
              } else {
                delete pathItem[method];
              }
            }
            delete pathItem.$ref;
            if (visible) visiblePaths[path] = pathItem;
          }
          content.paths = visiblePaths;
          delete content.webhooks;
          pruneUnusedComponents(content);
        }
        configuration.content = content;
      }
    }
  } catch {
  } finally {
    clearTimeout(schemaTimeout);
  }

  reference.dataset.configuration = JSON.stringify(configuration);
  const script = document.createElement('script');
  for (const attribute of loader.attributes) {
    if (attribute.name !== 'id' && attribute.name !== 'type') {
      script.setAttribute(attribute.name, attribute.value);
    }
  }
  script.src = loader.dataset.src;
  script.removeAttribute('data-src');
  loader.replaceWith(script);
})();