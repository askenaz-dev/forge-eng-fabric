"""OpenTelemetry tracing setup. Best-effort — disabled if OTLP endpoint unavailable."""

from __future__ import annotations

import contextlib

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

_initialized = False


def init_tracing(service_name: str, otlp_endpoint: str) -> trace.Tracer:
    global _initialized
    if not _initialized:
        provider = TracerProvider(resource=Resource.create({"service.name": service_name}))
        with contextlib.suppress(Exception):
            exporter = OTLPSpanExporter(endpoint=f"{otlp_endpoint.rstrip('/')}/v1/traces")
            provider.add_span_processor(BatchSpanProcessor(exporter))
        trace.set_tracer_provider(provider)
        _initialized = True
    return trace.get_tracer(service_name)
