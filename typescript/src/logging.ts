import { pino, type DestinationStream, type Logger } from 'pino';

export function createLogger(destination: DestinationStream): Logger {
  return pino(
    {
      timestamp: pino.stdTimeFunctions.isoTime,
      serializers: { err: pino.stdSerializers.errWithCause },
    },
    destination,
  );
}
